// Command orangeflix-dl downloads the episodes of the Orangeflix formats.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/revunix/orangeflix-dl/internal/cli"
)

// version is set through -ldflags.
var version = "dev"

func main() {
	cli.Version = version

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
