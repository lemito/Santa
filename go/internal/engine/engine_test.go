package engine

import (
	"context"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestStatusMappingIsExplicit(t *testing.T) {
	cases := []struct {
		status Status
		name   string
	}{
		{Success, "success"},
		{InvalidInput, "invalid_input"},
		{Unsatisfiable, "unsatisfiable"},
		{SearchLimitExceeded, "search_limit"},
		{Cancelled, "cancelled"},
		{InternalError, "internal_error"},
		{ResourceLimitExceeded, "resource_limit"},
	}
	for _, tc := range cases {
		if tc.status.String() != tc.name {
			t.Fatalf("status %d = %q, want %q", tc.status, tc.status.String(), tc.name)
		}
	}
	if Status(99).String() == Success.String() {
		t.Fatal("unknown status aliases success")
	}
}

func TestValidateAssignmentsRejectsMissingAndSelf(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	in := Input{Participants: []uuid.UUID{a, b}}
	if ValidateAssignments(in, map[uuid.UUID]uuid.UUID{a: a, b: b}) == nil {
		t.Fatal("self assignments accepted")
	}
	if ValidateAssignments(in, map[uuid.UUID]uuid.UUID{a: b}) == nil {
		t.Fatal("incomplete assignment accepted")
	}
}

func TestValidateAssignmentsRejectsUnknownDuplicateAndExclusion(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	in := Input{Participants: []uuid.UUID{a, b, c}, Exclusions: []Exclusion{{First: a, Second: b}}}
	cases := []map[uuid.UUID]uuid.UUID{
		{a: b, b: c, c: uuid.New()},
		{a: b, b: b, c: a},
		{a: b, b: a, c: c},
	}
	for i, assignments := range cases {
		if ValidateAssignments(in, assignments) == nil {
			t.Errorf("case %d accepted invalid assignment", i)
		}
	}
}

func TestNativeCancelledBeforeCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := (Native{}).Draw(ctx, Input{Participants: []uuid.UUID{uuid.New(), uuid.New()}})
	if result.Status != Cancelled || err == nil {
		t.Fatalf("status=%v err=%v, want cancelled", result.Status, err)
	}
}

func TestNativeDoesNotFillPartialBudgetWithDefaults(t *testing.T) {
	result, err := (Native{Budget: Budget{MaxAttempts: 1}}).Draw(context.Background(), Input{
		Participants: []uuid.UUID{uuid.New(), uuid.New()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ResourceLimitExceeded {
		t.Fatalf("status=%v, want resource limit", result.Status)
	}
}

func TestNativeCancellationDuringCall(t *testing.T) {
	const n = 80
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
	}
	in := Input{Participants: ids}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			adjacent := j == (i+1)%n || i == (j+1)%n
			if !adjacent {
				in.Exclusions = append(in.Exclusions, Exclusion{First: ids[i], Second: ids[j]})
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(2 * time.Millisecond)
		cancel()
	}()
	result, err := (Native{Budget: Budget{MaxAttempts: 1_000_000_000, MaxWork: 1_000_000_000_000, MaxMemoryBytes: 128 * 1024 * 1024}}).Draw(ctx, in)
	if err != nil && result.Status != Cancelled {
		t.Fatal(err)
	}
	if result.Status != Cancelled {
		t.Fatalf("status=%v, want cancelled", result.Status)
	}
}
