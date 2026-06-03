package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"hynor/banana-mw/internal/config"
)

// namedServer pairs an http.Server with a label used in logs and errors.
type namedServer struct {
	name   string
	server *http.Server
}

// newHTTPServer builds an *http.Server bound to addr with the shared timeouts.
func newHTTPServer(cfg *config.Config, addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeoutDuration(),
		ReadTimeout:       cfg.ReadTimeoutDuration(),
		WriteTimeout:      cfg.WriteTimeoutDuration(),
		IdleTimeout:       cfg.IdleTimeoutDuration(),
	}
}

// runHTTPServers starts every server concurrently and blocks until ctx is
// cancelled or any server stops with an error, then drains all of them within
// shutdown_timeout.
func runHTTPServers(ctx context.Context, cfg *config.Config, servers ...namedServer) error {
	if len(servers) == 0 {
		return errors.New("no http servers configured")
	}

	errCh := make(chan error, len(servers))
	for _, s := range servers {
		s := s
		slog.Info("http server listening", "service", s.name, "addr", s.server.Addr)
		go func() {
			if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("%s server: %w", s.name, err)
			}
		}()
	}

	var runErr error
	select {
	case runErr = <-errCh:
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeoutDuration())
	defer cancel()
	for _, s := range servers {
		if err := s.server.Shutdown(shutdownCtx); err != nil && runErr == nil {
			runErr = fmt.Errorf("%s shutdown: %w", s.name, err)
		}
	}
	return runErr
}
