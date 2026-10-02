package kit

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// onDisk: sistem berkasnya menyimpan izin eksekusi. Windows tidak.
const onDisk = runtime.GOOS != "windows"

// sample adalah identitas contoh kit uji, dengan bentuk yang sama seperti
// kit sungguhan.
var sample = Identity{
	ProductCode: "produk-contoh",
	DisplayName: "Produk Contoh",
	ModulePath:  "github.com/gonsu/starter",
}

const manifestJSON = `{
  "schema": 1,
  "kit": "uji",
  "label": "Kit Uji",
  "identity": {
    "product_code": "produk-contoh",
    "display_name": "Produk Contoh",
    "module_path": "github.com/gonsu/starter"
  },
  "remove": ["KIT.md", "scripts/kit-check.sh"],
  "install": [{ "dir": "web", "run": ["bun", "install"] }],
  "next_steps": ["make run   # server"]
}`

// kitFiles adalah isi kit uji: setiap bentuk berkas yang ditangani Apply.
var kitFiles = map[string]string{
	ManifestName:           manifestJSON,
	"KIT.md":               "Untuk perawat kit: produk-contoh.\n",
	"scripts/kit-check.sh": "#!/bin/sh\n",
	"scripts/run.sh":       "#!/bin/sh\nexec produk-contoh\n",
	"README.md": "# Produk Contoh\n\n<!-- gonsu-kit:begin -->\nHanya untuk kit.\n<!-- gonsu-kit:end -->\n\n" +
		"Kode `produk-contoh`, hak pakai `produk-contoh.core`, image `products/produk-contoh-web`.\n",
	"go.mod":             "module github.com/gonsu/starter\n",
	"cmd/api/main.go":    "package main\n\nimport _ \"github.com/gonsu/starter/internal/app\"\n\nconst code = \"produk-contoh\"\n",
	"web/lib/product.ts": "export const product = { code: \"produk-contoh\", name: \"Produk Contoh\" }\n",
	"web/logo.png":       "\x89PNG\x00\x00biner",
	"go.sum":             "example.com/lain v1.0.0 h1:abc=\n",
}

func writeKit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func apply(t *testing.T, dir string, id Identity) {
	t.Helper()
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(dir, m, id, Origin{Source: "https://example.com/kit.git", Version: "kit-v1.0.0", Commit: "abc123"}); err != nil {
		t.Fatal(err)
	}
}

func TestApply(t *testing.T) {
	dir := writeKit(t, kitFiles)
	apply(t, dir, Identity{ProductCode: "toko-baju", DisplayName: "Toko Baju", ModulePath: "github.com/organisasi/toko-baju"})

	for name, want := range map[string]string{
		// Nilai turunan mengikuti kode produk dengan sendirinya, dan bagian
		// khusus kit hilang tanpa meninggalkan baris kosong ganda.
		"README.md":          "# Toko Baju\n\nKode `toko-baju`, hak pakai `toko-baju.core`, image `products/toko-baju-web`.\n",
		"go.mod":             "module github.com/organisasi/toko-baju\n",
		"cmd/api/main.go":    "package main\n\nimport _ \"github.com/organisasi/toko-baju/internal/app\"\n\nconst code = \"toko-baju\"\n",
		"web/lib/product.ts": "export const product = { code: \"toko-baju\", name: \"Toko Baju\" }\n",
		"scripts/run.sh":     "#!/bin/sh\nexec toko-baju\n",
		"go.sum":             kitFiles["go.sum"],
		"web/logo.png":       kitFiles["web/logo.png"],
	} {
		if got := read(t, dir, name); got != want {
			t.Errorf("%s =\n%q\ningin\n%q", name, got, want)
		}
	}

	// Berkas milik kit dan manifesnya dibuang.
	for _, name := range []string{ManifestName, "KIT.md", "scripts/kit-check.sh"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil {
			t.Errorf("%s ikut ke project hasil", name)
		}
	}
	// Izin eksekusi skrip bertahan. Windows tidak menyimpannya di disk.
	if info, err := os.Stat(filepath.Join(dir, "scripts", "run.sh")); err != nil || (onDisk && info.Mode().Perm()&0o100 == 0) {
		t.Errorf("scripts/run.sh tidak lagi dapat dieksekusi: %v", err)
	}

	// Asal project tercatat, supaya selisih dengan tag kit berikutnya dapat dicari.
	var origin struct{ Kit, Source, Version, Commit string }
	if err := json.Unmarshal([]byte(read(t, dir, OriginPath)), &origin); err != nil {
		t.Fatal(err)
	}
	if origin.Kit != "uji" || origin.Version != "kit-v1.0.0" || origin.Commit != "abc123" {
		t.Errorf("asal = %+v", origin)
	}
}

// Penggantian satu lintasan: identitas produk yang MEMUAT nilai contoh tidak
// terganti dua kali.
func TestApplyReplacesInOnePass(t *testing.T) {
	dir := writeKit(t, kitFiles)
	apply(t, dir, Identity{
		ProductCode: "produk-contoh-2", DisplayName: "Produk Contoh Dua",
		ModulePath: "github.com/organisasi/produk-contoh-2",
	})
	if got := read(t, dir, "go.mod"); got != "module github.com/organisasi/produk-contoh-2\n" {
		t.Errorf("go.mod = %q", got)
	}
	want := "export const product = { code: \"produk-contoh-2\", name: \"Produk Contoh Dua\" }\n"
	if got := read(t, dir, "web/lib/product.ts"); got != want {
		t.Errorf("product.ts = %q", got)
	}
}

// Nama tampilan ditulis ter-escape di dalam string kode, dan apa adanya di
// Markdown.
func TestApplyEscapesDisplayNameInCode(t *testing.T) {
	dir := writeKit(t, kitFiles)
	apply(t, dir, Identity{ProductCode: "toko", DisplayName: `Toko "Baju" \ <Anak>`, ModulePath: "github.com/organisasi/toko"})

	want := `export const product = { code: "toko", name: "Toko \"Baju\" \\ <Anak>" }` + "\n"
	if got := read(t, dir, "web/lib/product.ts"); got != want {
		t.Errorf("product.ts = %s", got)
	}
	if got := read(t, dir, "README.md"); !strings.HasPrefix(got, `# Toko "Baju" \ <Anak>`+"\n") {
		t.Errorf("README.md = %q", got)
	}
}

func TestApplyRejections(t *testing.T) {
	id := Identity{ProductCode: "toko", DisplayName: "Toko", ModulePath: "github.com/organisasi/toko"}
	with := func(name, content string) map[string]string {
		files := map[string]string{}
		for k, v := range kitFiles {
			files[k] = v
		}
		files[name] = content
		return files
	}
	for name, tc := range map[string]struct {
		files map[string]string
		id    Identity
		want  string
	}{
		"nama berkas memuat identitas contoh":  {with("cmd/produk-contoh/main.go", "package main\n"), id, "nama berkas"},
		"berkas biner memuat identitas contoh": {with("web/icon.bin", "\x00produk-contoh"), id, "berkas biner"},
		"penanda kit tidak ditutup":            {with("AGENTS.md", "a\n<!-- gonsu-kit:begin -->\nb\n"), id, "gonsu-kit:begin"},
		"penanda penutup tanpa pembuka":        {with("AGENTS.md", "a\n<!-- gonsu-kit:end -->\n"), id, "gonsu-kit:end"},
		"kit Go tanpa module path":             {kitFiles, Identity{ProductCode: "toko", DisplayName: "Toko"}, "module path"},
		"tanpa kode produk":                    {kitFiles, Identity{DisplayName: "Toko"}, "wajib"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := writeKit(t, tc.files)
			m, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			err = Apply(dir, m, tc.id, Origin{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, ingin menyebut %q", err, tc.want)
			}
		})
	}
}

// Kit boleh membawa symlink, tetapi tidak ada yang dihapus atau ditulis Apply
// di luar folder project — dengan --install=false sekalipun, kit tidak
// menyentuh berkas pemakai.
func TestApplyStaysInsideProject(t *testing.T) {
	id := Identity{ProductCode: "toko", DisplayName: "Toko", ModulePath: "github.com/organisasi/toko"}
	// outside adalah folder milik pemakai di luar project, berisi satu berkas.
	outside := func(t *testing.T) (dir, file string) {
		t.Helper()
		dir = t.TempDir()
		file = filepath.Join(dir, "penting.txt")
		if err := os.WriteFile(file, []byte("milik pemakai"), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir, file
	}
	symlink := func(t *testing.T, target, link string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink tidak dapat dibuat di sistem ini: %v", err)
		}
	}
	untouched := func(t *testing.T, file string) {
		t.Helper()
		if got, err := os.ReadFile(file); err != nil || string(got) != "milik pemakai" {
			t.Errorf("berkas di luar project berubah: %q, %v", got, err)
		}
	}
	manifest := func(remove string) string {
		return `{"schema": 1, "kit": "uji", "identity": {"product_code": "produk-contoh", "display_name": "Produk Contoh"},
			"remove": [` + remove + `]}`
	}

	t.Run("remove lewat symlink", func(t *testing.T) {
		away, file := outside(t)
		dir := writeKit(t, map[string]string{ManifestName: manifest(`"keluar/penting.txt"`)})
		symlink(t, away, filepath.Join(dir, "keluar"))
		m, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(dir, m, id, Origin{}); err == nil || !strings.Contains(err.Error(), "keluar/penting.txt") {
			t.Errorf("err = %v, ingin menyebut jalur yang ditolak", err)
		}
		untouched(t, file)
	})

	t.Run("catatan asal berupa symlink", func(t *testing.T) {
		_, file := outside(t)
		dir := writeKit(t, map[string]string{ManifestName: manifest("")})
		symlink(t, file, filepath.Join(dir, filepath.FromSlash(OriginPath)))
		m, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(dir, m, id, Origin{Commit: "abc123"}); err != nil {
			t.Fatal(err)
		}
		untouched(t, file)
		// Yang ditulis berkas baru di dalam project.
		if info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(OriginPath))); err != nil || !info.Mode().IsRegular() {
			t.Errorf("%s bukan berkas biasa: %v", OriginPath, err)
		}
		if got := read(t, dir, OriginPath); !strings.Contains(got, "abc123") {
			t.Errorf("%s = %q", OriginPath, got)
		}
	})

	t.Run("folder catatan asal berupa symlink", func(t *testing.T) {
		away, _ := outside(t)
		dir := writeKit(t, map[string]string{ManifestName: manifest("")})
		symlink(t, away, filepath.Join(dir, filepath.Dir(filepath.FromSlash(OriginPath))))
		m, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(dir, m, id, Origin{}); err == nil || !strings.Contains(err.Error(), OriginPath) {
			t.Errorf("err = %v, ingin menyebut %s", err, OriginPath)
		}
		if _, err := os.Stat(filepath.Join(away, filepath.Base(OriginPath))); err == nil {
			t.Error("catatan asal ditulis di luar project")
		}
	})
}

func TestLoadRejections(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil || !strings.Contains(err.Error(), "bukan starter kit") {
		t.Errorf("folder tanpa manifes = %v", err)
	}
	for name, tc := range map[string]struct{ manifest, want string }{
		"bukan JSON":            {`{`, "tidak dapat dibaca"},
		"skema lebih baru":      {`{"schema": 2, "kit": "x", "identity": {"product_code": "a-b", "display_name": "A"}}`, "perbarui gonsu"},
		"tanpa skema":           {`{"kit": "x", "identity": {"product_code": "a-b", "display_name": "A"}}`, "schema"},
		"tanpa identitas":       {`{"schema": 1, "kit": "x"}`, "identity"},
		"kode bagian dari nama": {`{"schema": 1, "kit": "x", "identity": {"product_code": "kit", "display_name": "kit saya"}}`, "bagian dari"},
		"kode bagian dari module": {`{"schema": 1, "kit": "x", "identity": {"product_code": "starter", "display_name": "S", "module_path": "example.com/starter"}}`,
			"bagian dari"},
		"remove keluar folder": {`{"schema": 1, "kit": "x", "identity": {"product_code": "a-b", "display_name": "A"}, "remove": ["../milik-orang"]}`, "keluar"},
		"install tanpa perintah": {`{"schema": 1, "kit": "x", "identity": {"product_code": "a-b", "display_name": "A"}, "install": [{"dir": "."}]}`,
			"tanpa perintah"},
		"install keluar folder": {`{"schema": 1, "kit": "x", "identity": {"product_code": "a-b", "display_name": "A"}, "install": [{"dir": "/tmp", "run": ["x"]}]}`,
			"keluar"},
		// Yang dicetak ke terminal tidak boleh dapat menggeser kursor.
		"label memuat escape": {`{"schema": 1, "kit": "x", "label": "Kit \u001b[2K", "identity": {"product_code": "a-b", "display_name": "A"}}`,
			"karakter kontrol"},
		"install memuat escape": {`{"schema": 1, "kit": "x", "identity": {"product_code": "a-b", "display_name": "A"}, "install": [{"run": ["sh", "-c", "x\u001b[1A"]}]}`,
			"karakter kontrol"},
		"next_steps memuat baris baru": {`{"schema": 1, "kit": "x", "identity": {"product_code": "a-b", "display_name": "A"}, "next_steps": ["make run\nrm -rf"]}`,
			"karakter kontrol"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := writeKit(t, map[string]string{ManifestName: tc.manifest})
			if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, ingin menyebut %q", err, tc.want)
			}
		})
	}
	// Isian yang belum dikenal diabaikan: manifes boleh bertambah tanpa
	// menaikkan skema.
	dir := writeKit(t, map[string]string{ManifestName: `{"schema": 1, "kit": "x", "isian_baru": true,
		"identity": {"product_code": "a-b", "display_name": "A"}}`})
	if _, err := Load(dir); err != nil {
		t.Errorf("isian yang belum dikenal ditolak: %v", err)
	}
}

func TestStripKitBlocks(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"tanpa penanda":       {"a\nb\n", "a\nb\n"},
		"di antara dua baris": {"a\n# gonsu-kit:begin\nx\n# gonsu-kit:end\nb\n", "a\nb\n"},
		"diapit baris kosong": {"a\n\n<!-- gonsu-kit:begin -->\nx\n<!-- gonsu-kit:end -->\n\nb\n", "a\n\nb\n"},
		"dua bagian":          {"// gonsu-kit:begin\nx\n// gonsu-kit:end\na\n// gonsu-kit:begin\ny\n// gonsu-kit:end\nb\n", "a\nb\n"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := stripKitBlocks(tc.in)
			if err != nil || got != tc.want {
				t.Errorf("hasil = %q, %v; ingin %q", got, err, tc.want)
			}
		})
	}
}

// Folder biasa disalin seluruhnya; izin berkasnya ikut.
func TestFetchFromDirectory(t *testing.T) {
	src := writeKit(t, kitFiles)
	dst := filepath.Join(t.TempDir(), "toko")
	origin, err := Fetch(context.Background(), src, "", dst)
	if err != nil {
		t.Fatal(err)
	}
	// Path folder sumber tidak dicatat: ia path laptop seseorang.
	if origin.Source != LocalSource || origin.Version != "" {
		t.Errorf("asal = %+v", origin)
	}
	if got := read(t, dst, "go.mod"); got != kitFiles["go.mod"] {
		t.Errorf("go.mod = %q", got)
	}
	// Folder yang bukan repository git hanya punya izin di disk, dan Windows
	// tidak menyimpannya: di sana kit dari folder biasa tidak membawa izin
	// eksekusi. Kit dari git membawanya lewat indeks (TestFetchClones).
	if !onDisk {
		return
	}
	if info, err := os.Stat(filepath.Join(dst, "scripts", "run.sh")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("izin eksekusi tidak ikut tersalin: %v", err)
	}
	if got := strings.Join(sorted(origin.Executables), ","); got != "scripts/kit-check.sh,scripts/run.sh" {
		t.Errorf("berkas yang dapat dieksekusi = %q", got)
	}
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

func gitRepo(t *testing.T, dir string) func(args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git tidak ada")
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Identitas dan konfigurasi sendiri: test tidak bergantung pada
		// git config mesin yang menjalankannya.
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=uji", "GIT_AUTHOR_EMAIL=uji@example.invalid",
			"GIT_COMMITTER_NAME=uji", "GIT_COMMITTER_EMAIL=uji@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	return run
}

// Dari working tree repository git: berkas baru yang belum di-commit ikut,
// yang diabaikan tidak — perawat kit mencoba perubahannya sebelum commit.
func TestFetchFromWorkingTree(t *testing.T) {
	src := writeKit(t, kitFiles)
	run := gitRepo(t, src)
	if err := os.WriteFile(filepath.Join(src, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "awal")
	commit := run("rev-parse", "HEAD")

	for name, content := range map[string]string{"baru.txt": "belum di-commit", "node_modules/pkg/index.js": "diabaikan"} {
		path := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dst := filepath.Join(t.TempDir(), "toko")
	origin, err := Fetch(context.Background(), src, "", dst)
	if err != nil {
		t.Fatal(err)
	}
	if origin.Commit != commit {
		t.Errorf("commit = %q, ingin %q", origin.Commit, commit)
	}
	if _, err := os.Stat(filepath.Join(dst, "baru.txt")); err != nil {
		t.Error("berkas baru yang belum di-commit tidak ikut")
	}
	for _, name := range []string{"node_modules", ".git"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err == nil {
			t.Errorf("%s ikut tersalin", name)
		}
	}
}

// Dari alamat git: tag yang diminta, atau ujung cabang bawaan bila tidak ada
// yang diminta. Riwayat kit tidak ikut ke project.
func TestFetchClones(t *testing.T) {
	src := writeKit(t, kitFiles)
	run := gitRepo(t, src)
	run("add", "-A")
	// Seperti kit sungguhan: izin eksekusi dicatat di indeks git, sehingga
	// tetap ada walau repository-nya dibuat di Windows.
	run("update-index", "--chmod=+x", "scripts/kit-check.sh", "scripts/run.sh")
	run("commit", "-q", "-m", "awal")
	run("tag", "kit-v1.0.0")
	tagged := run("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(src, "sesudah-tag.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "sesudah tag")

	// file:// membuat git memperlakukannya sebagai alamat, bukan folder.
	source := "file://" + filepath.ToSlash(src)
	dst := filepath.Join(t.TempDir(), "toko")
	origin, err := Fetch(context.Background(), source, "kit-v1.0.0", dst)
	if err != nil {
		t.Fatal(err)
	}
	if origin.Source != source || origin.Version != "kit-v1.0.0" || origin.Commit != tagged {
		t.Errorf("asal = %+v, ingin commit %s", origin, tagged)
	}
	// Dibaca dari indeks git: di Windows izin eksekusi tidak ada di disk.
	if got := strings.Join(sorted(origin.Executables), ","); got != "scripts/kit-check.sh,scripts/run.sh" {
		t.Errorf("berkas yang dapat dieksekusi = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "sesudah-tag.txt")); err == nil {
		t.Error("yang diambil ujung cabang, bukan tag yang diminta")
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); err == nil {
		t.Error("riwayat git kit ikut ke project")
	}

	// Tanpa tag atau cabang: ujung cabang bawaan kit — inilah yang dipakai
	// gonsu new, supaya perubahan kit sampai tanpa rilis gonsu.
	latest := filepath.Join(t.TempDir(), "terbaru")
	origin, err = Fetch(context.Background(), source, "", latest)
	if err != nil {
		t.Fatal(err)
	}
	if origin.Version != "" || origin.Commit == tagged || origin.Commit == "" {
		t.Errorf("asal tanpa version = %+v, ingin commit sesudah %s", origin, tagged)
	}
	if _, err := os.Stat(filepath.Join(latest, "sesudah-tag.txt")); err != nil {
		t.Error("tanpa version, yang diambil bukan ujung cabang bawaan")
	}

	// Tag yang tidak ada: pesannya menyebut versinya, bukan petunjuk akses —
	// kit-nya terjangkau, hanya versinya yang salah.
	_, err = Fetch(context.Background(), source, "kit-v9.9.9", filepath.Join(t.TempDir(), "lain"))
	if err == nil || !strings.Contains(err.Error(), "versi kit-v9.9.9 tidak ada") || strings.Contains(err.Error(), "gh auth login") {
		t.Errorf("tag yang tidak ada = %v", err)
	}
	// Alamat yang tidak terjangkau: petunjuk akses, bukan galat git mentah.
	_, err = Fetch(context.Background(), "file://"+filepath.Join(t.TempDir(), "tidak-ada"), "", filepath.Join(t.TempDir(), "lain"))
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Errorf("kit yang tidak terjangkau = %v", err)
	}
}

// Token di alamat kit tidak ikut ke pesan galat maupun ke .gonsu/kit.json,
// yang masuk commit pertama project.
func TestWithoutCredentials(t *testing.T) {
	for source, want := range map[string]string{
		"https://github.com/organisasi/kit.git":                        "https://github.com/organisasi/kit.git",
		"https://x-access-token:rahasia@github.com/organisasi/kit.git": "https://github.com/organisasi/kit.git",
		"https://rahasia@github.com/organisasi/kit.git":                "https://github.com/organisasi/kit.git",
		"ssh://git@github.com/organisasi/kit.git":                      "ssh://git@github.com/organisasi/kit.git",
		"ssh://git:rahasia@github.com/organisasi/kit.git":              "ssh://git@github.com/organisasi/kit.git",
		"git@github.com:organisasi/kit.git":                            "git@github.com:organisasi/kit.git",
		"file:///srv/kit":                                              "file:///srv/kit",
	} {
		if got := withoutCredentials(source); got != want {
			t.Errorf("withoutCredentials(%q) = %q, ingin %q", source, got, want)
		}
	}
}
