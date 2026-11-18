package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"santa/go/internal/auth"
	"strings"
	"time"
)

type Game struct {
	ID, OwnerID  uuid.UUID
	Title, State string
	Version      int64
}
type Participant struct {
	GameID, AccountID uuid.UUID
	Email, Name       string
}
type Exclusion struct{ GameID, FirstID, SecondID uuid.UUID }
type Draw struct {
	GameID     uuid.UUID
	Version    int64
	RequestKey uuid.UUID
}
type Assignment struct{ GameID, GiverID, ReceiverID uuid.UUID }
type Snapshot struct {
	Game         Game
	Participants []uuid.UUID
	Exclusions   []Exclusion
}

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrForbidden = errors.New("forbidden")

type Store struct{ DB *sql.DB }

func (s *Store) CreateGame(ctx context.Context, owner uuid.UUID, title string) (uuid.UUID, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 150 {
		return uuid.Nil, fmt.Errorf("invalid title")
	}
	id := uuid.New()
	_, e := s.DB.ExecContext(ctx, `INSERT INTO "Games" ("Id","OwnerId","Title","State","Version") VALUES ($1,$2,$3,'Draft',1)`, id, owner, title)
	return id, e
}
func (s *Store) ListGames(ctx context.Context, actor uuid.UUID) ([]Game, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT g."Id",g."OwnerId",g."Title",g."State",g."Version" FROM "Games" g WHERE g."OwnerId"=$1 OR EXISTS(SELECT 1 FROM "Participants" p WHERE p."GameId"=g."Id" AND p."AccountId"=$1) ORDER BY g."Title"`, actor)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Game
	for rows.Next() {
		var g Game
		if e := rows.Scan(&g.ID, &g.OwnerID, &g.Title, &g.State, &g.Version); e != nil {
			return nil, e
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *Store) GetGame(ctx context.Context, actor, id uuid.UUID) (Game, error) {
	var g Game
	e := s.DB.QueryRowContext(ctx, `SELECT g."Id",g."OwnerId",g."Title",g."State",g."Version" FROM "Games" g WHERE g."Id"=$1 AND (g."OwnerId"=$2 OR EXISTS(SELECT 1 FROM "Participants" p WHERE p."GameId"=g."Id" AND p."AccountId"=$2))`, id, actor).Scan(&g.ID, &g.OwnerID, &g.Title, &g.State, &g.Version)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
	}
	return g, e
}
func (s *Store) Configure(ctx context.Context, actor, id uuid.UUID, version int64, emails []string, pairs [][2]string) error {
	if len(emails) < 2 {
		return fmt.Errorf("invalid configuration")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var owner uuid.UUID
	var state string
	var v int64
	e = tx.QueryRowContext(ctx, `SELECT "OwnerId","State","Version" FROM "Games" WHERE "Id"=$1 FOR UPDATE`, id).Scan(&owner, &state, &v)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	if owner != actor {
		return ErrForbidden
	}
	if state != "Draft" || v != version {
		return ErrConflict
	}
	norm := make([]string, len(emails))
	seen := map[string]bool{}
	for i, x := range emails {
		norm[i] = auth.NormalizeEmail(x)
		if norm[i] == "" || seen[norm[i]] {
			return fmt.Errorf("invalid participants")
		}
		seen[norm[i]] = true
	}
	rows, e := tx.QueryContext(ctx, `SELECT "Id","NormalizedEmail" FROM "AspNetUsers" WHERE "NormalizedEmail" = ANY($1)`, norm)
	if e != nil {
		return e
	}
	ids := map[string]uuid.UUID{}
	for rows.Next() {
		var x uuid.UUID
		var n string
		if e = rows.Scan(&x, &n); e != nil {
			rows.Close()
			return e
		}
		ids[n] = x
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	if len(ids) != len(norm) {
		return fmt.Errorf("unknown account")
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM "Exclusions" WHERE "GameId"=$1`, id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM "Participants" WHERE "GameId"=$1`, id); e != nil {
		return e
	}
	for _, n := range norm {
		if _, e = tx.ExecContext(ctx, `INSERT INTO "Participants" ("GameId","AccountId") VALUES ($1,$2)`, id, ids[n]); e != nil {
			return e
		}
	}
	for _, p := range pairs {
		a, oka := ids[auth.NormalizeEmail(p[0])]
		b, okb := ids[auth.NormalizeEmail(p[1])]
		if !oka || !okb || a == b {
			return fmt.Errorf("invalid exclusion")
		}
		if a.String() > b.String() {
			a, b = b, a
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO "Exclusions" ("GameId","FirstId","SecondId") VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, id, a, b); e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, `UPDATE "Games" SET "Version"="Version"+1 WHERE "Id"=$1`, id)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Capture(ctx context.Context, actor, id uuid.UUID, version int64, key uuid.UUID) (*Snapshot, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var g Game
	e = tx.QueryRowContext(ctx, `SELECT "Id","OwnerId","Title","State","Version" FROM "Games" WHERE "Id"=$1 FOR UPDATE`, id).Scan(&g.ID, &g.OwnerID, &g.Title, &g.State, &g.Version)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	if g.OwnerID != actor {
		return nil, ErrForbidden
	}
	var priorKey uuid.UUID
	var priorVersion int64
	e = tx.QueryRowContext(ctx, `SELECT "RequestKey","Version" FROM "Draws" WHERE "GameId"=$1`, id).Scan(&priorKey, &priorVersion)
	if e == nil {
		if priorKey == key && priorVersion == version {
			return nil, nil
		}
		return nil, ErrConflict
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	if g.State != "Draft" || g.Version != version {
		return nil, ErrConflict
	}
	rows, e := tx.QueryContext(ctx, `SELECT "AccountId" FROM "Participants" WHERE "GameId"=$1 ORDER BY "AccountId"`, id)
	if e != nil {
		return nil, e
	}
	var ps []uuid.UUID
	for rows.Next() {
		var x uuid.UUID
		if e = rows.Scan(&x); e != nil {
			rows.Close()
			return nil, e
		}
		ps = append(ps, x)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows, e = tx.QueryContext(ctx, `SELECT "GameId","FirstId","SecondId" FROM "Exclusions" WHERE "GameId"=$1`, id)
	if e != nil {
		return nil, e
	}
	var ex []Exclusion
	for rows.Next() {
		var x Exclusion
		if e = rows.Scan(&x.GameID, &x.FirstID, &x.SecondID); e != nil {
			rows.Close()
			return nil, e
		}
		ex = append(ex, x)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return &Snapshot{Game: g, Participants: ps, Exclusions: ex}, nil
}
func (s *Store) Commit(ctx context.Context, actor uuid.UUID, snap *Snapshot, version int64, key uuid.UUID, assign map[uuid.UUID]uuid.UUID) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var g Game
	e = tx.QueryRowContext(ctx, `SELECT "Id","OwnerId","Title","State","Version" FROM "Games" WHERE "Id"=$1 FOR UPDATE`, snap.Game.ID).Scan(&g.ID, &g.OwnerID, &g.Title, &g.State, &g.Version)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	if g.OwnerID != actor || g.State != "Draft" || g.Version != version {
		return ErrConflict
	}
	var prior uuid.UUID
	e = tx.QueryRowContext(ctx, `SELECT "RequestKey" FROM "Draws" WHERE "GameId"=$1`, g.ID).Scan(&prior)
	if e == nil {
		if prior == key {
			return nil
		}
		return ErrConflict
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO "Draws" ("GameId","Version","RequestKey","EngineVersion") VALUES ($1,$2,$3,'uniform-hybrid-v1')`, g.ID, version, key); e != nil {
		return e
	}
	for a, b := range assign {
		if _, e = tx.ExecContext(ctx, `INSERT INTO "Assignments" ("GameId","GiverId","ReceiverId") VALUES ($1,$2,$3)`, g.ID, a, b); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE "Games" SET "State"='Published' WHERE "Id"=$1`, g.ID); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Close(ctx context.Context, actor, id uuid.UUID) error {
	r, e := s.DB.ExecContext(ctx, `UPDATE "Games" SET "State"='Closed' WHERE "Id"=$1 AND "OwnerId"=$2 AND "State"='Published'`, id, actor)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	return nil
}
func (s *Store) Recipient(ctx context.Context, actor, id uuid.UUID) (string, string, string, error) {
	var name, email, message string
	e := s.DB.QueryRowContext(ctx, `SELECT u."DisplayName",u."Email",p."SantaMessage" FROM "Assignments" a JOIN "Games" g ON g."Id"=a."GameId" JOIN "AspNetUsers" u ON u."Id"=a."ReceiverId" JOIN "Participants" p ON p."GameId"=a."GameId" AND p."AccountId"=a."ReceiverId" WHERE a."GameId"=$1 AND a."GiverId"=$2 AND g."State"='Published'`, id, actor).Scan(&name, &email, &message)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
	}
	return name, email, message, e
}
func (s *Store) MyMessage(ctx context.Context, actor, id uuid.UUID) (string, error) {
	var message string
	e := s.DB.QueryRowContext(ctx, `SELECT p."SantaMessage" FROM "Participants" p JOIN "Games" g ON g."Id"=p."GameId" WHERE p."GameId"=$1 AND p."AccountId"=$2 AND g."State"<>'Closed'`, id, actor).Scan(&message)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
	}
	return message, e
}
func (s *Store) SetMessage(ctx context.Context, actor, id uuid.UUID, message string) error {
	message = strings.TrimSpace(message)
	if len([]rune(message)) > 5000 {
		return fmt.Errorf("invalid message")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var state string
	if e = tx.QueryRowContext(ctx, `SELECT "State" FROM "Games" WHERE "Id"=$1 FOR UPDATE`, id).Scan(&state); errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if state == "Closed" {
		return ErrConflict
	}
	r, e := tx.ExecContext(ctx, `UPDATE "Participants" SET "SantaMessage"=$3 WHERE "GameId"=$1 AND "AccountId"=$2`, id, actor, message)
	if e != nil {
		return e
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return ErrForbidden
	}
	return tx.Commit()
}
func (s *Store) MyRecipientMessage(ctx context.Context, actor, id uuid.UUID) (string, error) {
	var message string
	e := s.DB.QueryRowContext(ctx, `SELECT a."RecipientMessage" FROM "Assignments" a JOIN "Games" g ON g."Id"=a."GameId" WHERE a."GameId"=$1 AND a."GiverId"=$2 AND g."State"='Published'`, id, actor).Scan(&message)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
	}
	return message, e
}
func (s *Store) SetRecipientMessage(ctx context.Context, actor, id uuid.UUID, message string) error {
	message = strings.TrimSpace(message)
	if len([]rune(message)) > 5000 {
		return fmt.Errorf("invalid message")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var state string
	if e = tx.QueryRowContext(ctx, `SELECT "State" FROM "Games" WHERE "Id"=$1 FOR UPDATE`, id).Scan(&state); errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if state != "Published" {
		return ErrConflict
	}
	r, e := tx.ExecContext(ctx, `UPDATE "Assignments" SET "RecipientMessage"=$3 WHERE "GameId"=$1 AND "GiverId"=$2`, id, actor, message)
	if e != nil {
		return e
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return ErrForbidden
	}
	return tx.Commit()
}
func (s *Store) MessageFromSanta(ctx context.Context, actor, id uuid.UUID) (string, error) {
	var message string
	e := s.DB.QueryRowContext(ctx, `SELECT a."RecipientMessage" FROM "Assignments" a JOIN "Games" g ON g."Id"=a."GameId" WHERE a."GameId"=$1 AND a."ReceiverId"=$2 AND g."State"='Published'`, id, actor).Scan(&message)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
	}
	return message, e
}
func (s *Store) CleanupSessions(ctx context.Context) error {
	_, e := s.DB.ExecContext(ctx, `DELETE FROM "SantaGoSessions" WHERE "ExpiresAt" < $1`, time.Now().UTC())
	return e
}
