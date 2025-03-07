// Package suturehttp provides a [suture.Service] that manages a [http.Server].
package suturehttp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/thejerf/suture/v4"

	"github.com/andrew-d/upchek/internal/ulog"
)

// Server is a [suture.Service] that manages a [http.Server].
type Server struct {
	// Logger is the logger that the server will use to log messages.
	//
	// If this is not provided, the server will use the default logger.
	Logger *slog.Logger

	ln      net.Listener
	handler http.Handler
}

var _ suture.Service = (*Server)(nil)

// New creates a new [Server] that listens on the provided [net.Listener] and
// serves HTTP requests with the provided [http.Handler].
func New(ln net.Listener, handler http.Handler) *Server {
	return &Server{
		ln:      ln,
		handler: handler,
	}
}

// Serve implements the suture.Service interface.
func (s *Server) Serve(ctx context.Context) error {
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	if s.ln == nil {
		return fmt.Errorf("listener is nil")
	}

	srv := &http.Server{Handler: s.handler}

	// Start the server in a goroutine so that we can listen for context
	// cancellation.
	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(s.ln)
	}()

	// Wait for either the service's context to be cancelled or for the
	// server to shut down.
	select {
	case <-ctx.Done():
	case err := <-errc:
		s.Logger.Error("API server shut down unexpectedly", ulog.Error(err))
		return err
	}

	// If we get here, we know that the context we were provided has been
	// cancelled. We need to shut down the server gracefully.
	//
	// Give ourselves a few seconds to do that.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.Logger.Info("shutting down API")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		s.Logger.Warn("API did not shut down gracefully before timeout", ulog.Error(err))
		srv.Close()
	}
	return ctx.Err()
}
