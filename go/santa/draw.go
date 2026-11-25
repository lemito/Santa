// Package santa exposes the standalone Secret Santa assignment engine.
// It has no HTTP, database, identity, or template dependencies.
package santa

import (
	"context"

	"github.com/lemito/Santa/go/internal/engine"
)

type Status = engine.Status

type Exclusion = engine.Exclusion
type Input = engine.Input
type Budget = engine.Budget

type Result = engine.Result

const (
	Success               = engine.Success
	InvalidInput          = engine.InvalidInput
	Unsatisfiable         = engine.Unsatisfiable
	SearchLimitExceeded   = engine.SearchLimitExceeded
	Cancelled             = engine.Cancelled
	InternalError         = engine.InternalError
	ResourceLimitExceeded = engine.ResourceLimitExceeded
)

type Options struct {
	Budget Budget
}

func Draw(ctx context.Context, in Input, options Options) (Result, error) {
	return engine.Native{Budget: options.Budget}.Draw(ctx, in)
}
