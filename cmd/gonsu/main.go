// Command gonsu membuat produk GONSU One.
//
//	gonsu new [kode-produk]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"golang.org/x/term"

	"github.com/gonsutrijayautama/gonsu-cli/internal/cli"
)

// version diisi saat build rilis: -ldflags "-X main.version=v1.2.3".
var version string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gonsu:", err)
		os.Exit(1)
	}
	err = cli.Run(ctx, os.Args[1:], cli.Env{
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Interactive: term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
		Dir:         dir,
		Version:     version,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "gonsu:", err)
		if errors.Is(err, cli.ErrUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
