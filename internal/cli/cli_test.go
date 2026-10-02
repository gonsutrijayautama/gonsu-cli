package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gonsutrijayautama/gonsu-cli/internal/catalog"
	"github.com/gonsutrijayautama/gonsu-cli/internal/kit"
)

// kitFiles adalah starter kit uji: cukup untuk membuktikan gonsu new
// mengambil, mengganti identitas, dan menjalankan yang disebut manifesnya.
var kitFiles = map[string]string{
	kit.ManifestName: `{
  "schema": 1,
  "kit": "go-nextjs",
  "label": "Go + Next.js",
  "identity": {
    "product_code": "produk-contoh",
    "display_name": "Produk Contoh",
    "module_path": "github.com/gonsu/starter"
  },
  "remove": ["KIT.md", "scripts/kit-check.sh"],
  "install": [
    { "dir": ".", "run": ["go", "mod", "download"] },
    { "dir": "web", "run": ["bun", "install", "--frozen-lockfile"] }
  ],
  "next_steps": ["make run       # server dan database lokal"]
}`,
	"KIT.md":               "Untuk perawat kit.\n",
	"scripts/e2e.sh":       "#!/bin/sh\n",
	"scripts/kit-check.sh": "#!/bin/sh\n",
	"README.md":            "# Produk Contoh\n\nHak pakai `produk-contoh.core`, module `github.com/gonsu/starter`.\n",
	"go.mod":               "module github.com/gonsu/starter\n",
}

type fakeWorld struct {
	env      Env
	out      *bytes.Buffer
	commands []string
	missing  map[string]bool
	asked    bool
	// fetched mencatat alamat dan tag kit yang diminta.
	fetched  []string
	fetchErr error
}

func newWorld(t *testing.T, interactive bool) *fakeWorld {
	t.Helper()
	w := &fakeWorld{out: &bytes.Buffer{}, missing: map[string]bool{}}
	w.env = Env{
		Stdout: w.out, Stderr: w.out, Interactive: interactive,
		Dir: t.TempDir(), Version: "v9.9.9",
		Exec: func(_ context.Context, dir, name string, args ...string) error {
			w.commands = append(w.commands, filepath.Base(dir)+": "+name+" "+strings.Join(args, " "))
			return nil
		},
		LookPath: func(name string) (string, error) {
			if w.missing[name] {
				return "", errors.New("tidak ada")
			}
			return "/usr/bin/" + name, nil
		},
		// Tanpa jaringan: kit uji ditulis langsung ke folder tujuan.
		Fetch: func(_ context.Context, source, version, dir string) (kit.Origin, error) {
			w.fetched = append(w.fetched, source+"@"+version)
			if w.fetchErr != nil {
				// Seperti git clone yang gagal di tengah jalan.
				_ = os.MkdirAll(filepath.Join(dir, "setengah"), 0o755)
				return kit.Origin{}, w.fetchErr
			}
			for name, content := range kitFiles {
				path := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return kit.Origin{}, err
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					return kit.Origin{}, err
				}
			}
			return kit.Origin{Source: source, Version: version, Commit: "abc123",
				Executables: []string{"scripts/e2e.sh", "scripts/kit-check.sh"}}, nil
		},
	}
	return w
}

func TestNewNonInteractive(t *testing.T) {
	w := newWorld(t, false)
	// Kode produk sesudah flag pun diterima.
	err := Run(context.Background(), []string{"new", "--name", "Toko Baju Muslim", "toko-baju"}, w.env)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(w.env.Dir, "toko-baju")
	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Toko Baju Muslim", "`toko-baju.core`", "github.com/gonsutrijayautama/toko-baju"} {
		if !strings.Contains(string(readme), want) {
			t.Errorf("README tidak memuat %q:\n%s", want, readme)
		}
	}
	// Berkas milik kit tidak ikut; asal project tercatat.
	for _, name := range []string{"KIT.md", kit.ManifestName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s ikut ke project hasil", name)
		}
	}
	if origin, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(kit.OriginPath))); err != nil || !strings.Contains(string(origin), `"commit": "abc123"`) {
		t.Errorf("catatan asal = %q, %v", origin, err)
	}

	// Kit bawaan diambil dari katalog, pada ujung cabang bawaannya: perubahan
	// kit sampai ke gonsu new tanpa rilis gonsu.
	k, _ := catalog.FirstAvailable()
	if want := []string{k.Repository + "@"}; strings.Join(w.fetched, "\n") != strings.Join(want, "\n") {
		t.Errorf("kit yang diambil = %q, ingin %q", w.fetched, want)
	}
	// Git, lalu pemasangan dependency seperti disebut manifes kit. Skrip kit
	// ditandai dapat dieksekusi di indeks git (yang milik kit sudah dibuang,
	// jadi tidak ikut ditandai).
	want := []string{
		"toko-baju: git init -q -b main",
		"toko-baju: git add -A",
		"toko-baju: git update-index --chmod=+x -- scripts/e2e.sh",
		"toko-baju: git commit -q -m Awal dari gonsu new",
		"toko-baju: go mod download",
		"web: bun install --frozen-lockfile",
	}
	if strings.Join(w.commands, "\n") != strings.Join(want, "\n") {
		t.Errorf("perintah = %q", w.commands)
	}
	for _, want := range []string{"cd toko-baju", "make run", "# server dan database lokal", "Go + Next.js"} {
		if !strings.Contains(w.out.String(), want) {
			t.Errorf("keluaran tidak memuat %q:\n%s", want, w.out)
		}
	}
}

// Tanpa --version yang diambil keadaan terbaru kit; --version memilih versi
// tertentu. Nomor versi menjadi tag berawalan kit-, apa pun cara menulisnya.
func TestNewVersion(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"tanpa --version: terbaru": {nil, "gonsu-starter-go-nextjs.git@"},
		"nomor versi":              {[]string{"--version", "0.1.1"}, "gonsu-starter-go-nextjs.git@kit-v0.1.1"},
		"nomor versi berawalan v":  {[]string{"--version", "v0.1.1"}, "gonsu-starter-go-nextjs.git@kit-v0.1.1"},
		"pra-rilis":                {[]string{"--version", "1.0.0-rc.1"}, "gonsu-starter-go-nextjs.git@kit-v1.0.0-rc.1"},
		"tag ditulis lengkap":      {[]string{"--version", "kit-v0.1.1"}, "gonsu-starter-go-nextjs.git@kit-v0.1.1"},
		// Perawat kit: cabang yang belum digabung, atau kit dari tempat lain.
		"nama cabang":            {[]string{"--version", "feat/website"}, "gonsu-starter-go-nextjs.git@feat/website"},
		"sumber lain":            {[]string{"--kit-source", "/kit/lokal"}, "/kit/lokal@"},
		"sumber lain pada versi": {[]string{"--kit-source", "https://example.com/kit.git", "--version", "2.0.0"}, "https://example.com/kit.git@kit-v2.0.0"},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, false)
			args := append([]string{"new", "toko", "--git=false", "--install=false"}, tc.args...)
			if err := Run(context.Background(), args, w.env); err != nil {
				t.Fatal(err)
			}
			if len(w.fetched) != 1 || !strings.HasSuffix(w.fetched[0], tc.want) {
				t.Errorf("kit yang diambil = %q, ingin berakhiran %q", w.fetched, tc.want)
			}
		})
	}
}

func TestNewWithoutGitAndInstall(t *testing.T) {
	w := newWorld(t, false)
	if err := Run(context.Background(), []string{"new", "toko", "--git=false", "--install=false"}, w.env); err != nil {
		t.Fatal(err)
	}
	if len(w.commands) != 0 {
		t.Errorf("perintah dijalankan: %q", w.commands)
	}
}

// Tanpa git atau tanpa alat pemasang, project tetap dibuat dan pemakai
// diberi tahu apa yang perlu dijalankan nanti.
func TestNewWithMissingTools(t *testing.T) {
	w := newWorld(t, false)
	w.missing["git"], w.missing["bun"] = true, true
	if err := Run(context.Background(), []string{"new", "toko"}, w.env); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"git tidak ditemukan", "bun tidak ditemukan; jalankan bun install --frozen-lockfile di toko/web nanti"} {
		if !strings.Contains(w.out.String(), want) {
			t.Errorf("keluaran tidak memuat %q:\n%s", want, w.out)
		}
	}
}

func TestNewRejections(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		usage bool
		want  string
	}{
		{"tanpa kode di luar terminal", []string{"new"}, true, "kode produk wajib"},
		{"kode huruf besar", []string{"new", "Toko"}, false, "huruf kecil"},
		{"kit belum tersedia", []string{"new", "toko", "--kit", "laravel"}, false, "belum tersedia"},
		{"kit tidak dikenal", []string{"new", "toko", "--kit", "rust"}, false, "tidak dikenal"},
		{"module path tidak sah", []string{"new", "toko", "--module", "Toko Baju"}, false, "module path"},
		{"argumen berlebih", []string{"new", "toko", "lain"}, true, "berlebih"},
		{"flag tak dikenal", []string{"new", "--warna", "biru"}, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t, false)
			err := Run(context.Background(), tt.args, w.env)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, ingin menyebut %q", err, tt.want)
			}
			if errors.Is(err, ErrUsage) != tt.usage {
				t.Errorf("ErrUsage = %v, ingin %v", errors.Is(err, ErrUsage), tt.usage)
			}
			if entries, _ := os.ReadDir(w.env.Dir); len(entries) != 0 {
				t.Errorf("project tetap ditulis walau ditolak")
			}
			// Yang ditolak sebelum kit diambil tidak menyentuh jaringan.
			if len(w.fetched) != 0 {
				t.Errorf("kit tetap diambil: %q", w.fetched)
			}
		})
	}
}

// Kit yang gagal diambil tidak meninggalkan project setengah jadi.
func TestNewCleansUpWhenKitFails(t *testing.T) {
	w := newWorld(t, false)
	w.fetchErr = errors.New("repository tidak ditemukan")
	err := Run(context.Background(), []string{"new", "toko"}, w.env)
	if err == nil || !strings.Contains(err.Error(), "repository tidak ditemukan") {
		t.Fatalf("err = %v", err)
	}
	if entries, _ := os.ReadDir(w.env.Dir); len(entries) != 0 {
		t.Errorf("sisa kit yang gagal tertinggal: %v", entries)
	}
	if len(w.commands) != 0 {
		t.Errorf("perintah tetap dijalankan: %q", w.commands)
	}

	// Folder kosong milik pemakai dikembalikan kosong, tidak dihapus.
	w = newWorld(t, false)
	w.fetchErr = errors.New("gagal")
	existing := filepath.Join(w.env.Dir, "toko")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"new", "toko"}, w.env); err == nil {
		t.Fatal("galat kit tertelan")
	}
	if entries, err := os.ReadDir(existing); err != nil || len(entries) != 0 {
		t.Errorf("folder kosong milik pemakai = %v, %v", entries, err)
	}
}

// Di terminal, yang belum diberikan lewat flag ditanyakan; yang diberikan
// tidak. -n mematikan tanya-jawab sama sekali.
func TestNewAsksOnlyInTerminal(t *testing.T) {
	w := newWorld(t, true)
	w.env.Ask = func(o *newOptions) error {
		w.asked = true
		if !o.given["kit"] || o.given["name"] {
			t.Errorf("flag yang diberikan = %v", o.given)
		}
		o.code, o.name = "toko-roti", "Roti Enak"
		return nil
	}
	if err := Run(context.Background(), []string{"new", "--kit", "go-nextjs", "--install=false"}, w.env); err != nil {
		t.Fatal(err)
	}
	if !w.asked {
		t.Fatal("tidak bertanya di terminal")
	}
	readme, err := os.ReadFile(filepath.Join(w.env.Dir, "toko-roti", "README.md"))
	if err != nil || !strings.Contains(string(readme), "# Roti Enak") {
		t.Errorf("README = %q, %v", readme, err)
	}

	w = newWorld(t, true)
	w.env.Ask = func(*newOptions) error { w.asked = true; return nil }
	if err := Run(context.Background(), []string{"new", "toko", "-n", "--git=false", "--install=false"}, w.env); err != nil {
		t.Fatal(err)
	}
	if w.asked {
		t.Error("-n tetap bertanya")
	}
}

func TestNewRefusesExistingDir(t *testing.T) {
	w := newWorld(t, false)
	existing := filepath.Join(w.env.Dir, "toko")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(existing, "catatan.txt")
	if err := os.WriteFile(note, []byte("milik orang"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"new", "toko"}, w.env); !errors.Is(err, ErrNotEmpty) {
		t.Fatalf("folder yang sudah berisi = %v", err)
	}
	// Isinya tidak disentuh, dan kit tidak diambil.
	if raw, err := os.ReadFile(note); err != nil || string(raw) != "milik orang" {
		t.Errorf("berkas milik orang = %q, %v", raw, err)
	}
	if len(w.fetched) != 0 {
		t.Errorf("kit tetap diambil: %q", w.fetched)
	}
}

func TestVersionAndHelp(t *testing.T) {
	w := newWorld(t, false)
	if err := Run(context.Background(), []string{"version"}, w.env); err != nil || w.out.String() != "gonsu v9.9.9\n" {
		t.Errorf("version = %q, %v", w.out, err)
	}
	w.out.Reset()
	if err := Run(context.Background(), nil, w.env); err != nil || !strings.Contains(w.out.String(), "gonsu new") {
		t.Errorf("bantuan = %q, %v", w.out, err)
	}
	if err := Run(context.Background(), []string{"hapus"}, w.env); !errors.Is(err, ErrUsage) {
		t.Errorf("perintah tak dikenal = %v", err)
	}
}
