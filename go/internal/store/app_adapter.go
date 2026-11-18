package store

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"santa/go/internal/app"
	"santa/go/internal/engine"
)

// AppStore adapts the SQL store to the application port.
type AppStore struct{ *Store }

func (a AppStore) Create(ctx context.Context, actor uuid.UUID, title string) (uuid.UUID, error) {
	return a.CreateGame(ctx, actor, title)
}
func (a AppStore) List(ctx context.Context, actor uuid.UUID) ([]app.GameView, error) {
	gs, e := a.ListGames(ctx, actor)
	if e != nil {
		return nil, e
	}
	out := make([]app.GameView, 0, len(gs))
	for _, g := range gs {
		out = append(out, app.GameView{ID: g.ID, Title: g.Title, State: app.GameState(g.State), Version: g.Version, IsOwner: g.OwnerID == actor})
	}
	return out, nil
}
func (a AppStore) Get(ctx context.Context, actor, id uuid.UUID) (app.GameView, error) {
	g, e := a.GetGame(ctx, actor, id)
	if e != nil {
		return app.GameView{}, e
	}
	return app.GameView{ID: g.ID, Title: g.Title, State: app.GameState(g.State), Version: g.Version, IsOwner: g.OwnerID == actor}, nil
}
func (a AppStore) Configuration(ctx context.Context, actor, id uuid.UUID) (app.ConfigurationView, error) {
	var owner uuid.UUID
	if err := a.DB.QueryRowContext(ctx, `SELECT "OwnerId" FROM "Games" WHERE "Id"=$1`, id).Scan(&owner); err != nil {
		if err == sql.ErrNoRows {
			return app.ConfigurationView{}, ErrNotFound
		}
		return app.ConfigurationView{}, err
	}
	if owner != actor {
		return app.ConfigurationView{}, ErrForbidden
	}
	rows, e := a.DB.QueryContext(ctx, `SELECT p."AccountId",u."Email",u."DisplayName" FROM "Participants" p JOIN "AspNetUsers" u ON u."Id"=p."AccountId" WHERE p."GameId"=$1 ORDER BY p."AccountId"`, id)
	if e != nil {
		return app.ConfigurationView{}, e
	}
	defer rows.Close()
	var out app.ConfigurationView
	for rows.Next() {
		var id uuid.UUID
		var email, name string
		if e := rows.Scan(&id, &email, &name); e != nil {
			return out, e
		}
		out.Participants = append(out.Participants, app.ParticipantView{ID: id, Email: email, Name: name})
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	exRows, err := a.DB.QueryContext(ctx, `SELECT f."Email", s."Email" FROM "Exclusions" e JOIN "AspNetUsers" f ON f."Id"=e."FirstId" JOIN "AspNetUsers" s ON s."Id"=e."SecondId" WHERE e."GameId"=$1 ORDER BY f."Email", s."Email"`, id)
	if err != nil {
		return out, err
	}
	defer exRows.Close()
	for exRows.Next() {
		var first, second string
		if err := exRows.Scan(&first, &second); err != nil {
			return out, err
		}
		out.Exclusions = append(out.Exclusions, app.Exclusion{First: first, Second: second})
	}
	return out, exRows.Err()
}
func (a AppStore) Configure(ctx context.Context, actor, id uuid.UUID, in app.ConfigurationInput) error {
	pairs := make([][2]string, len(in.Exclusions))
	for i, p := range in.Exclusions {
		pairs[i] = [2]string{p.First, p.Second}
	}
	return a.Store.Configure(ctx, actor, id, in.Version, in.Emails, pairs)
}
func (a AppStore) Capture(ctx context.Context, actor, id uuid.UUID, in app.PublishInput) (*app.Snapshot, error) {
	s, e := a.Store.Capture(ctx, actor, id, in.Version, in.RequestKey)
	if e != nil || s == nil {
		return nil, e
	}
	input := app.EngineInput{Participants: s.Participants}
	for _, p := range s.Exclusions {
		input.Exclusions = append(input.Exclusions, engine.Exclusion{First: p.FirstID, Second: p.SecondID})
	}
	return &app.Snapshot{GameID: s.Game.ID, Version: s.Game.Version, Input: input}, nil
}
func (a AppStore) Commit(ctx context.Context, actor uuid.UUID, in app.PublishInput, s app.Snapshot, assign map[uuid.UUID]uuid.UUID) error {
	return a.Store.Commit(ctx, actor, &Snapshot{Game: Game{ID: s.GameID, Version: s.Version}, Participants: s.Input.Participants}, in.Version, in.RequestKey, assign)
}
func (a AppStore) Close(ctx context.Context, actor, id uuid.UUID) error {
	return a.Store.Close(ctx, actor, id)
}
func (a AppStore) MyAssignment(ctx context.Context, actor, id uuid.UUID) (app.RecipientView, error) {
	n, e, m, er := a.Recipient(ctx, actor, id)
	if er != nil {
		return app.RecipientView{}, er
	}
	return app.RecipientView{Name: n, Email: e, Message: m}, nil
}
func (a AppStore) MyMessage(ctx context.Context, actor, id uuid.UUID) (app.MessageView, error) {
	m, err := a.Store.MyMessage(ctx, actor, id)
	return app.MessageView{Markdown: m}, err
}
func (a AppStore) SetMessage(ctx context.Context, actor, id uuid.UUID, message string) error {
	return a.Store.SetMessage(ctx, actor, id, message)
}
func (a AppStore) MyRecipientMessage(ctx context.Context, actor, id uuid.UUID) (app.MessageView, error) {
	m, err := a.Store.MyRecipientMessage(ctx, actor, id)
	return app.MessageView{Markdown: m}, err
}
func (a AppStore) SetRecipientMessage(ctx context.Context, actor, id uuid.UUID, message string) error {
	return a.Store.SetRecipientMessage(ctx, actor, id, message)
}
func (a AppStore) MessageFromSanta(ctx context.Context, actor, id uuid.UUID) (app.MessageView, error) {
	m, err := a.Store.MessageFromSanta(ctx, actor, id)
	return app.MessageView{Markdown: m}, err
}
