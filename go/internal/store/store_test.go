package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCreateGameValidatesTitleBeforeDatabase(t *testing.T) {
	s := &Store{}
	for _, title := range []string{"", "   ", string(make([]byte, 151))} {
		if _, err := s.CreateGame(context.Background(), uuid.New(), title); err == nil {
			t.Errorf("title %q was accepted", title)
		}
	}
}

func TestPostgresConstraintsAndLifecycle(t *testing.T) {
	connection := os.Getenv("SANTA_TEST_DB")
	if connection == "" {
		t.Skip("SANTA_TEST_DB is not set")
	}
	db, err := sql.Open("pgx", connection)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `TRUNCATE TABLE "Assignments", "Draws", "Exclusions", "Participants", "Games", "AspNetUsers" CASCADE`); err != nil {
		t.Fatal(err)
	}
	owner, receiver := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, receiver} {
		email := id.String() + "@example.test"
		if _, err := db.ExecContext(ctx, `INSERT INTO "AspNetUsers" ("Id","Email","NormalizedEmail","DisplayName") VALUES ($1,$2,$3,$4)`, id, email, strings.ToUpper(email), id.String()); err != nil {
			t.Fatal(err)
		}
	}
	s := &Store{DB: db}
	game, err := s.CreateGame(ctx, owner, "constraints")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Configure(ctx, owner, game, 1, []string{owner.String() + "@example.test", receiver.String() + "@example.test"}, nil); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Capture(ctx, owner, game, 2, uuid.New())
	if err != nil || snapshot == nil {
		t.Fatalf("capture: %v", err)
	}
	assignments := map[uuid.UUID]uuid.UUID{owner: receiver, receiver: owner}
	if err := s.Commit(ctx, owner, snapshot, 2, uuid.New(), assignments); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Capture(ctx, owner, game, 2, uuid.New()); !errors.Is(err, ErrConflict) {
		t.Fatalf("second request error=%v, want conflict", err)
	}
}
