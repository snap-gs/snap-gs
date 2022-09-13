package main

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	mrand "math/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/snap-gs/snap-gs/internal/log"
	"github.com/snap-gs/snap-gs/public/cmd"
)

// init seeds math.rand from crypto.rand.
// https://stackoverflow.com/a/54491783
func init() {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	mrand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}

func main() {
	// Defers after os.Exit do not fire.
	if err := mainc(context.Background()); err != nil {
		log.Errorf(os.Stderr, "main: error: %+v", err)
		os.Exit(cmd.ExitCode(err))
	}
}

func mainc(mainctx context.Context) error {
	hardctx, hardcancel := signal.NotifyContext(mainctx, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGHUP)
	softctx, softcancel := signal.NotifyContext(hardctx, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(softctx)
	defer hardcancel()
	defer softcancel()
	defer cancel()
	c := cmd.NewCommand()
	if buildinfo, ok := debug.ReadBuildInfo(); ok {
		c.Version = buildinfo.Main.Version
		for i := range buildinfo.Settings {
			if buildinfo.Settings[i].Key == "vcs.revision" {
				c.Version = buildinfo.Settings[i].Value
				break
			}
		}
	}
	_, err := c.ExecuteContextC(ctx)
	if err != nil && hardctx.Err() == nil && softctx.Err() != nil {
		return nil
	}
	return err
}
