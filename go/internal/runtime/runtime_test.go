package runtime

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type fakeServer struct {
	serveStarted chan struct{}
	serveStop    chan struct{}
	shutdown     chan context.Context
}

func (s *fakeServer) Serve() error {
	close(s.serveStarted)
	<-s.serveStop
	return http.ErrServerClosed
}
func (s *fakeServer) Shutdown(ctx context.Context) error {
	s.shutdown <- ctx
	close(s.serveStop)
	return nil
}

func TestRunGracefulShutdown(t *testing.T) {
	server := &fakeServer{serveStarted: make(chan struct{}), serveStop: make(chan struct{}), shutdown: make(chan context.Context, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, server, 50*time.Millisecond) }()
	<-server.serveStarted
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
	select {
	case shutdownCtx := <-server.shutdown:
		if _, ok := shutdownCtx.Deadline(); !ok {
			t.Fatal("shutdown context has no deadline")
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown was not called")
	}
}

func TestRunReturnsServeError(t *testing.T) {
	want := errors.New("serve failed")
	server := &errorServer{err: want}
	if err := Run(context.Background(), server, time.Second); !errors.Is(err, want) {
		t.Fatalf("Run() error = %v, want %v", err, want)
	}
}

type errorServer struct{ err error }

func (s *errorServer) Serve() error                 { return s.err }
func (*errorServer) Shutdown(context.Context) error { return nil }
