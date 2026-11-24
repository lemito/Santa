package httpapi

import (
	"errors"
	"testing"

	"santa/go/internal/app"
)

func TestAppErrorUsesTypedErrors(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"not found", app.ErrNotFound, ErrNotFound},
		{"forbidden", app.ErrForbidden, ErrForbidden},
		{"conflict", app.ErrConflict, ErrConflict},
		{"validation", app.ErrValidation, ErrValidation},
		{"unknown", errors.New("database conflict while writing"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := appError(tc.in)
			if tc.want == nil {
				if got == ErrConflict || got == ErrValidation || got == ErrNotFound || got == ErrForbidden {
					t.Fatalf("unknown error was classified as %v", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("appError(%v)=%v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
