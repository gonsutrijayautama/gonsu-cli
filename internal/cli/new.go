package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/gonsutrijayautama/gonsu-cli/internal/catalog"
	"github.com/gonsutrijayautama/gonsu-cli/internal/kit"
	"github.com/gonsutrijayautama/gonsu-cli/internal/project"
)

// newOptions adalah isian gonsu new, dari flag dan dari tanya-jawab.
type newOptions struct {
	code, name, kit, module string
	// version memilih versi kit tertentu; kosong berarti yang terbaru.
	version string
	// kitSource mengganti alamat kit dari katalog: untuk perawat kit yang
	// mencoba perubahannya sebelum digabung.
	kitSource     string
	git, install  bool
	noInteraction bool
	// given mencatat flag yang diberikan: pertanyaannya tidak ditanyakan lagi.
	given map[string]bool
}

func parseNew(args []string, stderr io.Writer) (*newOptions, error) {
	o := &newOptions{given: map[string]bool{}}
	fs := flag.NewFlagSet("gonsu new", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.name, "name", "", "nama tampilan produk")
	fs.StringVar(&o.kit, "kit", "", "starter kit")
	fs.StringVar(&o.module, "module", "", "module path Go")
	fs.StringVar(&o.version, "version", "", "versi kit, misalnya 0.1.1; kosong berarti yang terbaru")
	fs.StringVar(&o.kitSource, "kit-source", "", "folder atau alamat git kit, menggantikan katalog")
	fs.BoolVar(&o.git, "git", true, "buat repository git")
	fs.BoolVar(&o.install, "install", true, "pasang dependency")
	fs.BoolVar(&o.noInteraction, "no-interaction", false, "jangan bertanya")
	fs.BoolVar(&o.noInteraction, "n", false, "jangan bertanya")

	// Kode produk boleh ditulis sebelum atau sesudah flag.
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUsage, err)
	}
	if fs.NArg() > 0 {
		o.code = fs.Arg(0)
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUsage, err)
		}
		if fs.NArg() > 0 {
			return nil, fmt.Errorf("%w: argumen berlebih %q", ErrUsage, strings.Join(fs.Args(), " "))
		}
	}
	fs.Visit(func(f *flag.Flag) { o.given[f.Name] = true })
	if o.given["n"] {
		o.given["no-interaction"] = true
	}
	return o, nil
}

// fillDefaults mengisi isian yang kosong dengan bawaan.
func (o *newOptions) fillDefaults() {
	if o.name == "" && o.code != "" {
		o.name = project.DefaultDisplayName(o.code)
	}
	if o.kit == "" {
		if k, ok := catalog.FirstAvailable(); ok {
			o.kit = k.ID
		}
	}
	if o.module == "" && o.code != "" {
		o.module = project.DefaultModulePath(o.code)
	}
}

func (o *newOptions) validate() error {
	var errs []error
	if err := project.ValidateProductCode(o.code); err != nil {
		errs = append(errs, err)
	}
	if err := project.ValidateDisplayName(o.name); err != nil {
		errs = append(errs, err)
	}
	if _, err := catalog.Resolve(o.kit); err != nil {
		errs = append(errs, err)
	}
	// Diperiksa sebelum kit diambil: apakah kit-nya memakai module path baru
	// diketahui dari manifesnya, dan salah ketik tidak perlu menunggu unduhan.
	if err := project.ValidateModulePath(o.module); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// source adalah alamat kit yang diambil dan tag atau cabangnya. ref kosong
// berarti keadaan terbaru kit: ujung cabang bawaannya.
func (o *newOptions) source() (source, ref string) {
	k, _ := catalog.Lookup(o.kit)
	source = k.Repository
	if o.kitSource != "" {
		source = o.kitSource
	}
	return source, kitRef(o.version)
}

// versionNumber: 0.1.1 atau v0.1.1, dengan akhiran pra-rilis bila ada.
var versionNumber = regexp.MustCompile(`^v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)$`)

// kitRef menerjemahkan --version menjadi tag atau cabang kit.
//
// Nomor versi menjadi tag berawalan `kit-`: tag kit tidak pernah `v*` polos,
// yang di repository kit memicu pipeline rilis produk. Selain nomor versi —
// nama cabang, atau tag yang ditulis lengkap — dipakai apa adanya.
func kitRef(version string) string {
	version = strings.TrimSpace(version)
	if m := versionNumber.FindStringSubmatch(version); m != nil {
		return "kit-v" + m[1]
	}
	return version
}

func runNew(ctx context.Context, args []string, env Env) error {
	o, err := parseNew(args, env.Stderr)
	if err != nil {
		return err
	}
	ask := env.Interactive && !o.noInteraction
	if !ask && o.code == "" {
		return fmt.Errorf("%w: kode produk wajib diisi — gonsu new <kode-produk>", ErrUsage)
	}
	if ask {
		out := newPrinter(env.Stdout)
		_, _ = fmt.Fprintf(env.Stdout, "\n  %s\n\n", out.title.Render("GONSU — membuat produk baru"))
		if err := env.Ask(o); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return errors.New("dibatalkan; tidak ada yang ditulis")
			}
			return err
		}
	}
	o.fillDefaults()
	if err := o.validate(); err != nil {
		return err
	}

	out := newPrinter(env.Stdout)
	dir := filepath.Join(env.Dir, o.code)
	existed, err := ensureEmpty(dir)
	if err != nil {
		return err
	}
	manifest, origin, err := create(ctx, env, o, dir)
	if err != nil {
		// Kit yang gagal diambil atau diterapkan tidak meninggalkan project
		// setengah jadi. Folder kosong milik pemakai dikembalikan kosong.
		if existed {
			clearDir(dir)
		} else {
			_ = os.RemoveAll(dir)
		}
		return err
	}
	out.done("project dibuat di ./" + o.code + " dari kit " + manifest.Label)

	if o.git {
		gitInit(ctx, env, dir, origin.Executables, out)
	}
	if o.install {
		installDependencies(ctx, env, dir, o, manifest, out)
	}
	out.nextSteps(o, manifest)
	return nil
}

// create mengambil kit ke dir lalu menjadikannya project produk.
func create(ctx context.Context, env Env, o *newOptions, dir string) (kit.Manifest, kit.Origin, error) {
	source, version := o.source()
	origin, err := env.Fetch(ctx, source, version, dir)
	if err != nil {
		return kit.Manifest{}, kit.Origin{}, err
	}
	manifest, err := kit.Load(dir)
	if err != nil {
		return kit.Manifest{}, kit.Origin{}, err
	}
	id := kit.Identity{ProductCode: o.code, DisplayName: strings.TrimSpace(o.name)}
	if manifest.Identity.ModulePath != "" {
		id.ModulePath = o.module
	}
	if err := kit.Apply(dir, manifest, id, origin); err != nil {
		return kit.Manifest{}, kit.Origin{}, err
	}
	return manifest, origin, nil
}

// ErrNotEmpty berarti folder tujuan sudah berisi.
var ErrNotEmpty = errors.New("folder tujuan sudah berisi")

// ensureEmpty menolak folder tujuan yang sudah berisi: project orang tidak
// pernah ditimpa. existed melaporkan apakah foldernya sudah ada (kosong).
func ensureEmpty(dir string) (existed bool, err error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(entries) > 0 {
		return true, fmt.Errorf("%w: %s", ErrNotEmpty, dir)
	}
	return true, nil
}

func clearDir(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

// gitInit membuat repository git project beserta commit pertamanya.
// executables adalah skrip kit yang dapat dieksekusi.
func gitInit(ctx context.Context, env Env, dir string, executables []string, out printer) {
	if _, err := env.LookPath("git"); err != nil {
		out.warn("git tidak ditemukan; repository tidak dibuat")
		return
	}
	steps := [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}}
	// Izin eksekusi ditandai di indeks git, bukan hanya di disk: di Windows
	// disk tidak menyimpannya, dan skrip yang tercatat 100644 gagal dijalankan
	// CI project (`make e2e`, `make smoke`). Di sistem lain ini tidak
	// mengubah apa pun.
	if kept := existing(dir, executables); len(kept) > 0 {
		steps = append(steps, append([]string{"update-index", "--chmod=+x", "--"}, kept...))
	}
	for _, args := range steps {
		if err := env.Exec(ctx, dir, "git", args...); err != nil {
			out.warn("git " + args[0] + " gagal: " + firstLine(err))
			return
		}
	}
	// Commit pertama butuh identitas git; tanpa itu repository tetap dibuat.
	if err := env.Exec(ctx, dir, "git", "commit", "-q", "-m", "Awal dari gonsu new"); err != nil {
		out.warn("repository dibuat tanpa commit pertama (atur git config user.name dan user.email)")
		return
	}
	out.done("repository git dibuat")
}

// existing menyaring files ke yang masih ada di dir: berkas milik kit sudah
// dibuang sebelum git dibuat.
func existing(dir string, files []string) []string {
	var out []string
	for _, f := range files {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err == nil && info.Mode().IsRegular() {
			out = append(out, f)
		}
	}
	return out
}

// installDependencies menjalankan perintah pemasangan yang disebut manifes
// kit. Perintah yang gagal atau alatnya tidak ada hanya diperingatkan:
// project-nya sudah jadi, dan pemasangan dapat diulang.
func installDependencies(ctx context.Context, env Env, dir string, o *newOptions, manifest kit.Manifest, out printer) {
	for _, s := range manifest.Install {
		tool, args := s.Run[0], s.Run[1:]
		where := filepath.Join(dir, filepath.FromSlash(s.Dir))
		label := strings.Join(s.Run, " ")
		if _, err := env.LookPath(tool); err != nil {
			out.warn(tool + " tidak ditemukan; jalankan " + label + " di " + filepath.Join(o.code, s.Dir) + " nanti")
			continue
		}
		if err := env.Exec(ctx, where, tool, args...); err != nil {
			out.warn(label + " gagal: " + firstLine(err))
			continue
		}
		out.done(label)
	}
}

func firstLine(err error) string {
	s, _, _ := strings.Cut(err.Error(), "\n")
	return s
}

// ---------------------------------------------------------------------------
// Tanya-jawab di terminal
// ---------------------------------------------------------------------------

// askInTerminal menanyakan isian yang belum diberikan lewat flag.
func askInTerminal(o *newOptions) error {
	if o.code == "" {
		err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Kode produk").
				Description("Harus sama persis dengan kode produk di Console GONSU.").
				Placeholder("toko-baju").Value(&o.code).Validate(project.ValidateProductCode),
		)).Run()
		if err != nil {
			return err
		}
	}
	o.fillDefaults()

	var groups []*huh.Group
	if !o.given["name"] {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().Title("Nama tampilan").Value(&o.name).Validate(project.ValidateDisplayName)))
	}
	// Kit dari --kit-source tidak dipilih dari katalog.
	if !o.given["kit"] && o.kitSource == "" {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().Title("Starter kit").Options(kitOptions()...).Value(&o.kit).
				Validate(func(id string) error {
					_, err := catalog.Resolve(id)
					return err
				})))
	}
	if !o.given["module"] {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().Title("Module path Go").Value(&o.module).Validate(project.ValidateModulePath),
		).WithHideFunc(func() bool { return !strings.HasPrefix(o.kit, "go-") }))
	}
	var confirms []huh.Field
	if !o.given["git"] {
		confirms = append(confirms, huh.NewConfirm().Title("Buat repository git?").
			Affirmative("Ya").Negative("Tidak").Value(&o.git))
	}
	if !o.given["install"] {
		confirms = append(confirms, huh.NewConfirm().Title("Pasang dependency sekarang?").
			Affirmative("Ya").Negative("Tidak").Value(&o.install))
	}
	if len(confirms) > 0 {
		groups = append(groups, huh.NewGroup(confirms...))
	}
	if len(groups) == 0 {
		return nil
	}
	return huh.NewForm(groups...).Run()
}

func kitOptions() []huh.Option[string] {
	var opts []huh.Option[string]
	for _, k := range catalog.Kits {
		label := k.Label
		if !k.Available {
			label += "  (" + k.Pending + ")"
		}
		opts = append(opts, huh.NewOption(label, k.ID))
	}
	return opts
}

// ---------------------------------------------------------------------------
// Keluaran
// ---------------------------------------------------------------------------

type printer struct {
	w          io.Writer
	ok, attn   lipgloss.Style
	dim, title lipgloss.Style
}

func newPrinter(w io.Writer) printer {
	r := lipgloss.NewRenderer(w)
	return printer{
		w:     w,
		ok:    r.NewStyle().Foreground(lipgloss.Color("2")),
		attn:  r.NewStyle().Foreground(lipgloss.Color("3")),
		dim:   r.NewStyle().Faint(true),
		title: r.NewStyle().Bold(true),
	}
}

func (p printer) done(msg string) { _, _ = fmt.Fprintf(p.w, "  %s %s\n", p.ok.Render("✓"), msg) }
func (p printer) warn(msg string) { _, _ = fmt.Fprintf(p.w, "  %s %s\n", p.attn.Render("!"), msg) }

// nextSteps mencetak langkah berikutnya, dari manifes kit. Bagian sesudah
// " # " pada tiap baris adalah keterangannya.
func (p printer) nextSteps(o *newOptions, manifest kit.Manifest) {
	_, _ = fmt.Fprintf(p.w, "\n  %s\n    cd %s\n", p.title.Render("Langkah berikutnya:"), o.code)
	for _, line := range manifest.NextSteps {
		command, note, found := strings.Cut(line, " # ")
		if found {
			note = p.dim.Render("# " + note)
		}
		_, _ = fmt.Fprintf(p.w, "    %s %s\n", command, note)
	}
	_, _ = fmt.Fprintf(p.w, "  %s\n", p.dim.Render(fmt.Sprintf(
		"Sebelum rilis pertama: daftarkan produk %q di Console GONSU, lalu ikuti README.md.", o.code)))
}
