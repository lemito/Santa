//go:build !linux || !cgo

package engine

import (
	"context"
	"github.com/google/uuid"
)

func nativeDraw(context.Context, []uuid.UUID, []Exclusion, Budget) (Result, error) {
	return Result{Status: InternalError}, ErrUnsupportedPlatform
}
