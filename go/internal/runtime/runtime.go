package runtime

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type Server interface {
	Serve() error
	Shutdown(context.Context) error
}

func Run(ctx context.Context, server Server, shutdownTimeout time.Duration) error {
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve() }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
