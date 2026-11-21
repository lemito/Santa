package engine

import (
	"context"
	"github.com/google/uuid"
	"testing"
)

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
