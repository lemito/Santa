package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID          string
	Email       string
	DisplayName string
}
type Session struct {
	User *User
	CSRF string
}
type Auth interface {
	Session(*http.Request) (*Session, error)
	Register(context.Context, string, string, string) error
	Login(context.Context, string, string) (*User, error)
	Logout(context.Context, *http.Request) error
	ValidateCSRF(*http.Request) error
}
type Game struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	State            string `json:"state"`
	Version          int64  `json:"version"`
	Owner            bool   `json:"owner"`
	ParticipantCount int    `json:"participantCount"`
}
type Configuration struct {
	Version      int64         `json:"version"`
	Emails       []string      `json:"emails"`
	Participants []Participant `json:"participants"`
	Exclusions   []Pair        `json:"exclusions"`
}
type Participant struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}
type Pair struct {
	First  string `json:"first"`
	Second string `json:"second"`
}
type Recipient struct {
	Recipient Participant `json:"recipient"`
}
type PublishRequest struct {
	Version    int64  `json:"version"`
	RequestKey string `json:"requestKey"`
}
type GameService interface {
	List(context.Context, *User) ([]Game, error)
	Get(context.Context, *User, string) (Game, error)
	Configuration(context.Context, *User, string) (Configuration, error)
	Create(context.Context, *User, string) (string, error)
	Configure(context.Context, *User, string, Configuration, int64) error
	Publish(context.Context, *User, string, PublishRequest) error
	Close(context.Context, *User, string) error
	Assignment(context.Context, *User, string) (Recipient, error)
}
type Server struct {
	Auth      Auth
	Games     GameService
	Rate      *RateLimiter
	Static    http.Handler
	Templates *Templates
	DrawGate  chan struct{}
	Ready     func(context.Context) bool
}
type RateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}
func (r *RateLimiter) Allow(key string) bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	old := r.hits[key]
	i := 0
	for i < len(old) && now.Sub(old[i]) > r.window {
		i++
	}
	old = old[i:]
	if len(old) >= r.limit {
		return false
	}
	r.hits[key] = append(old, now)
	return true
}

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrValidation = errors.New("validation")

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func parseExclusions(raw string) ([]Pair, error) {
	var pairs []Pair
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil, errors.New("invalid_exclusion")
		}
		first, second := normalizeEmail(parts[0]), normalizeEmail(parts[1])
		if first == "" || second == "" {
			return nil, errors.New("invalid_exclusion")
		}
		pairs = append(pairs, Pair{First: first, Second: second})
	}
	return pairs, nil
}
