package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"santa/go/internal/engine"
)

type GameState string

const (
	Draft     GameState = "Draft"
	Published GameState = "Published"
	Closed    GameState = "Closed"
)

type Exclusion struct{ First, Second string }
type ConfigurationInput struct {
	Version    int64
	Emails     []string
	Exclusions []Exclusion
}
type PublishInput struct {
	Version    int64
	RequestKey uuid.UUID
}
type EngineInput = engine.Input
type Snapshot struct {
	GameID  uuid.UUID
	Version int64
	Input   engine.Input
}
type GameView struct {
	ID      uuid.UUID
	Title   string
	State   GameState
	Version int64
	IsOwner bool
}
type ParticipantView struct {
	ID          uuid.UUID
	Email, Name string
}
type ConfigurationView struct {
	Participants []ParticipantView
	Exclusions   []Exclusion
}
type RecipientView struct{ Name, Email, Message string }
type MessageView struct{ Markdown string }

var (
	ErrNotFound            = errors.New("not_found")
	ErrForbidden           = errors.New("forbidden")
	ErrConflict            = errors.New("configuration_conflict")
	ErrValidation          = errors.New("validation")
	ErrUnsatisfiable       = errors.New("unsatisfiable")
	ErrSearchLimit         = errors.New("search_limit")
	ErrResourceLimit       = errors.New("resource_limit")
	ErrCancelled           = errors.New("cancelled")
	ErrInternal            = errors.New("internal_error")
	ErrInvalidEngineResult = errors.New("invalid_engine_result")
)

type Store interface {
	Create(context.Context, uuid.UUID, string) (uuid.UUID, error)
	List(context.Context, uuid.UUID) ([]GameView, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (GameView, error)
	Configuration(context.Context, uuid.UUID, uuid.UUID) (ConfigurationView, error)
	Configure(context.Context, uuid.UUID, uuid.UUID, ConfigurationInput) error
	Capture(context.Context, uuid.UUID, uuid.UUID, PublishInput) (*Snapshot, error)
	Commit(context.Context, uuid.UUID, PublishInput, Snapshot, map[uuid.UUID]uuid.UUID) error
	Close(context.Context, uuid.UUID, uuid.UUID) error
	MyAssignment(context.Context, uuid.UUID, uuid.UUID) (RecipientView, error)
	MyMessage(context.Context, uuid.UUID, uuid.UUID) (MessageView, error)
	SetMessage(context.Context, uuid.UUID, uuid.UUID, string) error
	MyRecipientMessage(context.Context, uuid.UUID, uuid.UUID) (MessageView, error)
	SetRecipientMessage(context.Context, uuid.UUID, uuid.UUID, string) error
	MessageFromSanta(context.Context, uuid.UUID, uuid.UUID) (MessageView, error)
}

type Service struct {
	Store  Store
	Engine engine.Engine
}

func (s Service) Create(ctx context.Context, actor uuid.UUID, title string) (uuid.UUID, error) {
	return s.Store.Create(ctx, actor, title)
}
func (s Service) List(ctx context.Context, actor uuid.UUID) ([]GameView, error) {
	return s.Store.List(ctx, actor)
}
func (s Service) Get(ctx context.Context, actor, id uuid.UUID) (GameView, error) {
	return s.Store.Get(ctx, actor, id)
}
func (s Service) Configuration(ctx context.Context, actor, id uuid.UUID) (ConfigurationView, error) {
	return s.Store.Configuration(ctx, actor, id)
}
func (s Service) Configure(ctx context.Context, actor, id uuid.UUID, in ConfigurationInput) error {
	return s.Store.Configure(ctx, actor, id, in)
}
func (s Service) Close(ctx context.Context, actor, id uuid.UUID) error {
	return s.Store.Close(ctx, actor, id)
}
func (s Service) MyAssignment(ctx context.Context, actor, id uuid.UUID) (RecipientView, error) {
	return s.Store.MyAssignment(ctx, actor, id)
}
func (s Service) MyMessage(ctx context.Context, actor, id uuid.UUID) (MessageView, error) {
	return s.Store.MyMessage(ctx, actor, id)
}
func (s Service) SetMessage(ctx context.Context, actor, id uuid.UUID, message string) error {
	return s.Store.SetMessage(ctx, actor, id, message)
}
func (s Service) MyRecipientMessage(ctx context.Context, actor, id uuid.UUID) (MessageView, error) {
	return s.Store.MyRecipientMessage(ctx, actor, id)
}
func (s Service) SetRecipientMessage(ctx context.Context, actor, id uuid.UUID, message string) error {
	return s.Store.SetRecipientMessage(ctx, actor, id, message)
}
func (s Service) MessageFromSanta(ctx context.Context, actor, id uuid.UUID) (MessageView, error) {
	return s.Store.MessageFromSanta(ctx, actor, id)
}

func (s Service) Publish(ctx context.Context, actor, id uuid.UUID, request PublishInput) error {
	if request.RequestKey == uuid.Nil {
		return fmt.Errorf("invalid_request_key")
	}
	snapshot, err := s.Store.Capture(ctx, actor, id, request)
	if err != nil || snapshot == nil {
		return err
	}
	result, err := s.Engine.Draw(ctx, snapshot.Input)
	if err != nil {
		return err
	}
	if result.Status != engine.Success {
		return statusError(result.Status)
	}
	if err := engine.ValidateAssignments(snapshot.Input, result.Assignments); err != nil {
		return ErrInvalidEngineResult
	}
	return s.Store.Commit(ctx, actor, request, *snapshot, result.Assignments)
}

func statusError(status engine.Status) error {
	switch status {
	case engine.Unsatisfiable:
		return ErrUnsatisfiable
	case engine.SearchLimitExceeded:
		return ErrSearchLimit
	case engine.ResourceLimitExceeded:
		return ErrResourceLimit
	case engine.InvalidInput:
		return fmt.Errorf("%w: invalid_configuration", ErrValidation)
	case engine.Cancelled:
		return ErrCancelled
	default:
		return ErrInternal
	}
}
