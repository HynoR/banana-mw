package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"hynor/banana-mw/internal/server"
)

var buildVersion = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server.MustRun(ctx, buildVersion)
}
