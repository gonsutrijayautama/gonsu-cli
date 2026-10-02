package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"golang.org/x/mod/semver"
)

// Releases adalah halaman rilis gonsu: versi terbarunya, dan cara memasangnya
// menggantikan gonsu yang sedang berjalan.
type Releases interface {
	Latest(ctx context.Context) (string, error)
	Install(ctx context.Context, version string) error
}

// errNoReleases berarti gonsu ini dibangun tanpa akses ke halaman rilis.
var errNoReleases = errors.New("gonsu ini tidak dapat memeriksa rilis")

const installHint = "Pasang rilis lewat installer: https://github.com/gonsutrijayautama/gonsu-cli#memasang"

func runUpdate(ctx context.Context, args []string, env Env) error {
	fs := flag.NewFlagSet("gonsu update", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	fs.Usage = func() {}
	check := fs.Bool("check", false, "hanya memeriksa")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printHelp(env.Stdout)
			return nil
		}
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: gonsu update tidak menerima argumen %q", ErrUsage, fs.Arg(0))
	}
	if env.Releases == nil {
		return errNoReleases
	}

	out := newPrinter(env.Stdout)
	current := env.version()
	// Jaringan yang tidak menjawab tidak boleh membuat gonsu tampak macet.
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	latest, err := env.Releases.Latest(checkCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("memeriksa rilis terbaru: %w", err)
	}
	if !semver.IsValid(latest) {
		return fmt.Errorf("rilis terbaru bernomor %q, bukan nomor versi", latest)
	}

	switch {
	case !semver.IsValid(current):
		// `go run`, atau `go build` tanpa nomor versi: tidak ada yang dapat
		// dibandingkan, dan menimpanya diam-diam bukan yang diminta.
		if *check {
			_, _ = fmt.Fprintf(env.Stdout, "  gonsu ini build pengembangan %s; rilis terbaru %s.\n  %s\n", current, latest, installHint)
			return nil
		}
		return fmt.Errorf("gonsu ini build pengembangan %s dan tidak diperbarui otomatis; rilis terbaru %s.\n%s", current, latest, installHint)
	case semver.Compare(current, latest) >= 0:
		out.done("gonsu " + current + " sudah yang terbaru")
		return nil
	case *check:
		out.warn("gonsu " + latest + " tersedia (terpasang " + current + "). Jalankan: gonsu update")
		return nil
	}

	out.step("memperbarui gonsu " + current + " → " + latest)
	if err := env.Releases.Install(ctx, latest); err != nil {
		return fmt.Errorf("memperbarui gonsu: %w", err)
	}
	out.done("gonsu " + latest + " terpasang")
	return nil
}

// newerRelease memeriksa di latar belakang apakah ada rilis gonsu yang lebih
// baru, untuk diberitahukan sesudah `gonsu new` selesai. Hasilnya dibaca lewat
// fungsi yang dikembalikan; kosong berarti tidak ada, atau tidak diketahui.
//
// Pemeriksaan ini tidak pernah menggagalkan maupun menahan `gonsu new`:
// jaringan yang lambat atau mati hanya berarti tidak ada pemberitahuan.
func newerRelease(ctx context.Context, env Env) func() string {
	none := func() string { return "" }
	current := env.version()
	// Hanya di terminal: log CI tidak butuh ajakan memperbarui.
	if env.Releases == nil || !env.Interactive || !semver.IsValid(current) {
		return none
	}
	result := make(chan string, 1)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	go func() {
		latest, err := env.Releases.Latest(ctx)
		if err == nil && semver.IsValid(latest) && semver.Compare(current, latest) < 0 {
			result <- latest
			return
		}
		result <- ""
	}()
	return func() string {
		defer cancel()
		select {
		case latest := <-result:
			return latest
		case <-ctx.Done():
			return ""
		}
	}
}
