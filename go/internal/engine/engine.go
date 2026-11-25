package engine

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"sort"
)

type Exclusion struct{ First, Second uuid.UUID }
type Input struct {
	Participants []uuid.UUID
	Exclusions   []Exclusion
}
type Budget struct{ MaxAttempts, MaxWork, MaxMemoryBytes uint64 }
type Result struct {
	Status      Status
	Assignments map[uuid.UUID]uuid.UUID
}

type Engine interface {
	Draw(context.Context, Input) (Result, error)
}

var (
	ErrInvalidResult       = errors.New("invalid engine result")
	ErrUnsupportedPlatform = errors.New("native engine requires linux cgo")
)

// ValidateAssignments checks the native result independently of the engine.
func ValidateAssignments(in Input, assignments map[uuid.UUID]uuid.UUID) error {
	if len(in.Participants) < 2 || len(assignments) != len(in.Participants) {
		return ErrInvalidResult
	}
	ids := make(map[uuid.UUID]struct{}, len(in.Participants))
	for _, id := range in.Participants {
		if id == uuid.Nil {
			return ErrInvalidResult
		}
		if _, ok := ids[id]; ok {
			return ErrInvalidResult
		}
		ids[id] = struct{}{}
	}
	receivers := make(map[uuid.UUID]struct{}, len(assignments))
	for giver, receiver := range assignments {
		if _, ok := ids[giver]; !ok {
			return ErrInvalidResult
		}
		if _, ok := ids[receiver]; !ok || giver == receiver {
			return ErrInvalidResult
		}
		if _, ok := receivers[receiver]; ok {
			return ErrInvalidResult
		}
		receivers[receiver] = struct{}{}
	}
	for _, p := range in.Exclusions {
		if p.First == uuid.Nil || p.Second == uuid.Nil || p.First == p.Second {
			return ErrInvalidResult
		}
		if _, ok := ids[p.First]; !ok {
			return ErrInvalidResult
		}
		if _, ok := ids[p.Second]; !ok {
			return ErrInvalidResult
		}
		if assignments[p.First] == p.Second || assignments[p.Second] == p.First {
			return ErrInvalidResult
		}
	}
	return nil
}

type Native struct{ Budget Budget }

func (n Native) Draw(ctx context.Context, in Input) (Result, error) {
	if err := validateInput(in); err != nil {
		return Result{Status: InvalidInput}, err
	}
	select {
	case <-ctx.Done():
		return Result{Status: Cancelled}, ctx.Err()
	default:
	}
	if n.Budget == (Budget{}) {
		n.Budget = DefaultBudget()
	}
	if uint64(len(in.Participants)) > uint64(^uint32(0)) {
		return Result{Status: ResourceLimitExceeded}, fmt.Errorf("too many participants")
	}
	if len(in.Exclusions) > int(^uint(0)>>1)/2 {
		return Result{Status: ResourceLimitExceeded}, fmt.Errorf("too many exclusions")
	}
	ids := append([]uuid.UUID(nil), in.Participants...)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return nativeDraw(ctx, ids, in.Exclusions, n.Budget)
}

func DefaultBudget() Budget {
	return Budget{MaxAttempts: 100000, MaxWork: 100000000, MaxMemoryBytes: 128 * 1024 * 1024}
}

func validateInput(in Input) error {
	if len(in.Participants) < 2 {
		return fmt.Errorf("at least two participants required")
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range in.Participants {
		if id == uuid.Nil || seen[id] {
			return fmt.Errorf("duplicate or empty participant")
		}
		seen[id] = true
	}
	for _, p := range in.Exclusions {
		if p.First == uuid.Nil || p.Second == uuid.Nil || p.First == p.Second || !seen[p.First] || !seen[p.Second] {
			return fmt.Errorf("invalid exclusion")
		}
	}
	return nil
}
