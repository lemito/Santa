package app

import (
	"context"
	"github.com/google/uuid"
	"santa/go/internal/engine"
	"testing"
)

type fakeStore struct {
	snap      *Snapshot
	committed map[uuid.UUID]uuid.UUID
}

func (f *fakeStore) Create(context.Context, uuid.UUID, string) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (f *fakeStore) List(context.Context, uuid.UUID) ([]GameView, error) { return nil, nil }
func (f *fakeStore) Get(context.Context, uuid.UUID, uuid.UUID) (GameView, error) {
	return GameView{}, nil
}
func (f *fakeStore) Configuration(context.Context, uuid.UUID, uuid.UUID) (ConfigurationView, error) {
	return ConfigurationView{}, nil
}
func (f *fakeStore) Configure(context.Context, uuid.UUID, uuid.UUID, ConfigurationInput) error {
	return nil
}
func (f *fakeStore) Capture(context.Context, uuid.UUID, uuid.UUID, PublishInput) (*Snapshot, error) {
	return f.snap, nil
}
func (f *fakeStore) Commit(_ context.Context, _ uuid.UUID, _ PublishInput, _ Snapshot, assignments map[uuid.UUID]uuid.UUID) error {
	f.committed = assignments
	return nil
}
func (f *fakeStore) Close(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (f *fakeStore) MyAssignment(context.Context, uuid.UUID, uuid.UUID) (RecipientView, error) {
	return RecipientView{}, nil
}
func (f *fakeStore) MyMessage(context.Context, uuid.UUID, uuid.UUID) (MessageView, error) {
	return MessageView{}, nil
}
func (f *fakeStore) SetMessage(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
func (f *fakeStore) MyRecipientMessage(context.Context, uuid.UUID, uuid.UUID) (MessageView, error) {
	return MessageView{}, nil
}
func (f *fakeStore) SetRecipientMessage(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (f *fakeStore) MessageFromSanta(context.Context, uuid.UUID, uuid.UUID) (MessageView, error) {
	return MessageView{}, nil
}

type fakeEngine struct{ result engine.Result }

func (f fakeEngine) Draw(context.Context, engine.Input) (engine.Result, error) { return f.result, nil }

func TestPublishRejectsInvalidEngineResult(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	in := engine.Input{Participants: []uuid.UUID{a, b, c}}
	st := &fakeStore{snap: &Snapshot{GameID: uuid.New(), Version: 1, Input: in}}
	svc := Service{Store: st, Engine: fakeEngine{result: engine.Result{Status: engine.Success, Assignments: map[uuid.UUID]uuid.UUID{a: b, b: a, c: c}}}}
	if err := svc.Publish(context.Background(), uuid.New(), uuid.New(), PublishInput{Version: 1, RequestKey: uuid.New()}); err != ErrInvalidEngineResult {
		t.Fatalf("got %v", err)
	}
}

func TestPublishCommitsValidResult(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	in := engine.Input{Participants: []uuid.UUID{a, b, c}}
	st := &fakeStore{snap: &Snapshot{GameID: uuid.New(), Version: 1, Input: in}}
	result := map[uuid.UUID]uuid.UUID{a: b, b: c, c: a}
	svc := Service{Store: st, Engine: fakeEngine{result: engine.Result{Status: engine.Success, Assignments: result}}}
	if err := svc.Publish(context.Background(), uuid.New(), uuid.New(), PublishInput{Version: 1, RequestKey: uuid.New()}); err != nil {
		t.Fatal(err)
	}
}
