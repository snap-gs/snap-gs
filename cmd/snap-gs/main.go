package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/snap-gs/snap-gs/internal/log"
	"github.com/snap-gs/snap-gs/public/cmd"
)

func main() {
	// Defers after os.Exit do not fire.
	if err := mainc(context.Background()); err != nil {
		log.Errorf(os.Stderr, "main: err=%+v", err)
		os.Exit(1)
	}
}

func mainc(mainctx context.Context) error {
	hardctx, hardcancel := signal.NotifyContext(mainctx, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGHUP)
	softctx, softcancel := signal.NotifyContext(hardctx, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(softctx)
	defer hardcancel()
	defer softcancel()
	defer cancel()
	_, err := cmd.NewCommand().ExecuteContextC(ctx)
	if err != nil && hardctx.Err() == nil && softctx.Err() != nil {
		return nil
	}
	return err
}
