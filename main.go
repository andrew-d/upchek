package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"github.com/thejerf/suture/v4"
	"github.com/thejerf/sutureslog"

	"github.com/andrew-d/upchek/internal/buildtags"
	"github.com/andrew-d/upchek/internal/ulog"
)

var (
	flagVerbose = pflag.BoolP("verbose", "v", false, "verbose output")
)

func main() {
	pflag.Parse()

	// We're using slog for logging.
	logger := slog.Default()
	if *flagVerbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	if buildtags.IsDev {
		logger = logger.With(slog.Bool("dev", true))
	}

	supervisor := suture.New("upchek", suture.Spec{
		EventHook: (&sutureslog.Handler{Logger: logger}).MustHook(),
	})

	// TODO: add services

	// Now that we've set up our supervision tree, we can start it.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errc := supervisor.ServeBackground(ctx)
	logger.Info("supervisor started")
	err := <-errc
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("supervisor exited with error", ulog.Error(err))
		os.Exit(1)
	}

	logger.Info("supervisor exited cleanly")
}
