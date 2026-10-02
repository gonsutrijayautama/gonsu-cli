// Package cli adalah perintah-perintah gonsu.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime/debug"

	"github.com/gonsutrijayautama/gonsu-cli/internal/kit"
)

// Env adalah dunia luar yang dipakai perintah. main mengisinya dengan yang
// sungguhan; test menggantinya.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	// Interactive: stdin dan stdout terminal, sehingga boleh bertanya.
	Interactive bool
	// Dir adalah folder kerja; project baru dibuat di dalamnya.
	Dir string
	// Fetch menyalin starter kit ke dir. Nil berarti kit.Fetch: git clone
	// ujung cabang bawaan kit (atau tag/cabang version), atau salinan folder
	// untuk --kit-source lokal.
	Fetch func(ctx context.Context, source, version, dir string) (kit.Origin, error)
	// Exec menjalankan perintah di dir. Nil berarti os/exec.
	Exec func(ctx context.Context, dir, name string, args ...string) error
	// LookPath mencari perintah di PATH. Nil berarti exec.LookPath.
	LookPath func(name string) (string, error)
	// Ask menanyakan isian yang belum ada. Nil berarti formulir terminal.
	Ask func(o *newOptions) error
	// Version adalah versi gonsu, diisi saat build. Kosong berarti dibaca
	// dari informasi build Go (go install ...@v1.2.3).
	Version string
	// Releases adalah halaman rilis gonsu, untuk `gonsu update` dan
	// pemberitahuan versi baru. Nil berarti tidak diperiksa — bentuk yang
	// dipakai test, yang tidak pernah menghubungi jaringan.
	Releases Releases
}

// ErrUsage berarti argumen salah; pesannya sudah menjelaskan.
var ErrUsage = errors.New("argumen salah")

// Run menjalankan gonsu dengan argumen tanpa nama program.
func Run(ctx context.Context, args []string, env Env) error {
	env = env.withDefaults()
	if len(args) == 0 {
		printHelp(env.Stdout)
		return nil
	}
	switch args[0] {
	case "new":
		return runNew(ctx, args[1:], env)
	case "update":
		return runUpdate(ctx, args[1:], env)
	case "version", "--version", "-v":
		_, err := fmt.Fprintf(env.Stdout, "gonsu %s\n", env.version())
		return err
	case "help", "--help", "-h":
		printHelp(env.Stdout)
		return nil
	default:
		printHelp(env.Stderr)
		return fmt.Errorf("%w: perintah %q tidak dikenal", ErrUsage, args[0])
	}
}

func (e Env) withDefaults() Env {
	if e.Exec == nil {
		e.Exec = func(ctx context.Context, dir, name string, args ...string) error {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s %v: %w\n%s", name, args, err, out)
			}
			return nil
		}
	}
	if e.LookPath == nil {
		e.LookPath = exec.LookPath
	}
	if e.Fetch == nil {
		e.Fetch = kit.Fetch
	}
	if e.Ask == nil {
		e.Ask = askInTerminal
	}
	return e
}

func (e Env) version() string {
	if e.Version != "" {
		return e.Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

func printHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `gonsu — membuat produk GONSU One

Pemakaian:
  gonsu new [kode-produk] [flag]   membuat project baru
  gonsu update                     memperbarui gonsu ke rilis terbaru
  gonsu update --check             hanya memeriksa apakah ada rilis baru
  gonsu version                    versi gonsu

Flag gonsu new:
  --name <nama>        nama tampilan produk
  --kit <id>           starter kit: go-nextjs
  --version <versi>    versi kit, misalnya 0.1.1; tanpa ini yang terbaru
  --module <path>      module path Go, misalnya github.com/organisasi/toko
  --git=false          tanpa repository git
  --install=false      tanpa memasang dependency
  -n, --no-interaction jangan bertanya; isian yang kosong memakai bawaan

Untuk perawat starter kit:
  --kit-source <sumber>  folder atau alamat git kit, menggantikan katalog
  --version <cabang>     selain nomor versi, --version menerima nama cabang kit

Tanpa -n dan di terminal, gonsu menanyakan isian yang belum diberikan.
Tanpa --version, yang diambil keadaan TERBARU kit. Kit asal project tercatat
di .gonsu/kit.json. Starter kit diambil dengan git; kalau kit-nya privat, akun
GitHub kamu harus punya akses dan git harus bisa masuk (lihat README gonsu-cli).
`)
}
