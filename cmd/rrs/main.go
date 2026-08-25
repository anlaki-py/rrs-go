package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/anlaki-py/rrs-go/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(cli.Main(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
