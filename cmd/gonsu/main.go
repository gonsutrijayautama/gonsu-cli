// Command gonsu membuat produk GONSU One.
//
//	gonsu new [kode-produk]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"

	"golang.org/x/term"

	"github.com/gonsutrijayautama/gonsu-cli/internal/cli"
	"github.com/gonsutrijayautama/gonsu-cli/internal/selfupdate"
)

// version diisi saat build rilis: -ldflags "-X main.version=v1.2.3".
var version string

// restart menjalankan gonsu di exe dengan argumen args, tersambung ke terminal
// yang sama, dan menunggunya selesai.
//
// Tanpa konteks: Ctrl+C dari terminal sampai sendiri ke gonsu yang baru, yang
// lalu membersihkan project setengah jadi. Mematikannya dari sini memotong
// pembersihan itu.
func restart(exe string) func(context.Context, []string) error {
	return func(_ context.Context, args []string) error {
		cmd := exec.Command(exe, args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		err := cmd.Run()
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return cli.ExitError{Code: exit.ExitCode()}
		}
		return err
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gonsu:", err)
		os.Exit(1)
	}
	env := cli.Env{
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Interactive: term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
		Dir:         dir,
		Version:     version,
		// GONSU_RELEASES_URL mengganti alamat rilis — hanya untuk menguji
		// `gonsu update` terhadap server lokal.
		Releases: selfupdate.Client{Base: os.Getenv("GONSU_RELEASES_URL")},
	}
	// Dibaca sebelum ada yang diganti: sesudah pembaruan, binary baru berada
	// di path yang sama. Tanpa path ini gonsu new tidak memperbarui dirinya.
	if exe, err := os.Executable(); err == nil {
		env.Restart = restart(exe)
	}
	err = cli.Run(ctx, os.Args[1:], env)
	if err != nil {
		// Gonsu yang dijalankan ulang sudah mencetak galatnya sendiri.
		if exit, ok := errors.AsType[cli.ExitError](err); ok {
			os.Exit(max(exit.Code, 1))
		}
		fmt.Fprintln(os.Stderr, "gonsu:", err)
		if errors.Is(err, cli.ErrUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
