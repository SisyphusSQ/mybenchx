package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/SisyphusSQ/mybenchx/internal/cli"
)

func main() {
	signal.Ignore(syscall.SIGPIPE) // Report stdout failures with exit code 5.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
