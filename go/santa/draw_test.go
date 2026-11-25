package santa_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lemito/Santa/go/santa"
)

func TestDrawReturnsPermutationWithoutSelfAssignment(t *testing.T) {
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	result, err := santa.Draw(context.Background(), santa.Input{
		Participants: []uuid.UUID{a, b, c, d},
		Exclusions:   []santa.Exclusion{{First: a, Second: b}},
	}, santa.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != santa.Success || len(result.Assignments) != 4 {
		t.Fatalf("result=%+v", result)
	}
	if result.Assignments[a] == a || result.Assignments[a] == b {
		t.Fatalf("invalid recipient for a: %v", result.Assignments[a])
	}
}
