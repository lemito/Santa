package httpapi

import (
	"context"
	"strconv"
	"sync"
)

type MemoryGames struct {
	mu    sync.Mutex
	games map[string]Game
	cfg   map[string]Configuration
	next  int
}

func NewMemoryGames() *MemoryGames {
	return &MemoryGames{games: map[string]Game{}, cfg: map[string]Configuration{}}
}
func (g *MemoryGames) List(_ context.Context, u *User) ([]Game, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := []Game{}
	for _, v := range g.games {
		v.Owner = v.Owner || u != nil
		out = append(out, v)
	}
	return out, nil
}
func (g *MemoryGames) Get(_ context.Context, _ *User, id string) (Game, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	v, ok := g.games[id]
	if !ok {
		return Game{}, ErrNotFound
	}
	return v, nil
}
func (g *MemoryGames) Configuration(_ context.Context, _ *User, id string) (Configuration, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	v, ok := g.cfg[id]
	if !ok {
		return Configuration{}, ErrNotFound
	}
	return v, nil
}
func (g *MemoryGames) Create(_ context.Context, _ *User, title string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	id := strconv.Itoa(g.next)
	g.games[id] = Game{ID: id, Title: title, State: "Draft", Version: 1}
	g.cfg[id] = Configuration{}
	return id, nil
}
func (g *MemoryGames) Configure(_ context.Context, _ *User, id string, c Configuration, _ int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.games[id]; !ok {
		return ErrNotFound
	}
	g.cfg[id] = c
	return nil
}
func (g *MemoryGames) Publish(_ context.Context, _ *User, id string, _ PublishRequest) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	v, ok := g.games[id]
	if !ok {
		return ErrNotFound
	}
	v.State = "Published"
	g.games[id] = v
	return nil
}
func (g *MemoryGames) Close(_ context.Context, _ *User, id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	v, ok := g.games[id]
	if !ok {
		return ErrNotFound
	}
	v.State = "Closed"
	g.games[id] = v
	return nil
}
func (g *MemoryGames) Assignment(_ context.Context, _ *User, id string) (Recipient, error) {
	if _, e := g.Get(context.Background(), nil, id); e != nil {
		return Recipient{}, e
	}
	return Recipient{}, nil
}
func (g *MemoryGames) MyMessage(_ context.Context, _ *User, id string) (Message, error) {
	if _, e := g.Get(context.Background(), nil, id); e != nil {
		return Message{}, e
	}
	return Message{}, nil
}
func (g *MemoryGames) SetMessage(_ context.Context, _ *User, id, _ string) error {
	_, e := g.Get(context.Background(), nil, id)
	return e
}
func (g *MemoryGames) MyRecipientMessage(_ context.Context, _ *User, id string) (Message, error) {
	if _, e := g.Get(context.Background(), nil, id); e != nil {
		return Message{}, e
	}
	return Message{}, nil
}
func (g *MemoryGames) SetRecipientMessage(_ context.Context, _ *User, id, _ string) error {
	_, e := g.Get(context.Background(), nil, id)
	return e
}
func (g *MemoryGames) MessageFromSanta(_ context.Context, _ *User, id string) (Message, error) {
	if _, e := g.Get(context.Background(), nil, id); e != nil {
		return Message{}, e
	}
	return Message{}, nil
}
