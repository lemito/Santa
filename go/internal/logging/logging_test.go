package logging

import "testing"

func TestNewRejectsUnknownLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "not-a-level")
	logger, err := New()
	if err == nil || logger != nil {
		t.Fatalf("logger=%v err=%v, want configuration error", logger, err)
	}
}

func TestNewAcceptsConfiguredLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	logger, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()
}
