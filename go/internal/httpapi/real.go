package httpapi

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"santa/go/internal/auth"
)

// SQLAuth bridges the PostgreSQL Identity/session ports to the HTTP contract.
type SQLAuth struct {
	Users    auth.UserStore
	Sessions auth.Manager
}

func (a SQLAuth) SecureCookies() bool { return a.Sessions.Secure }

func (a SQLAuth) Session(r *http.Request) (*Session, error) {
	s, err := a.Sessions.Current(r.Context(), r)
	if err != nil {
		return nil, nil
	}
	u, err := a.Users.ByID(r.Context(), s.UserID)
	if err != nil {
		return nil, err
	}
	csrf := ""
	if c, e := r.Cookie("santa_go_csrf"); e == nil {
		csrf = c.Value
	}
	return &Session{User: &User{ID: u.ID.String(), Email: u.Email, DisplayName: u.DisplayName}, CSRF: csrf}, nil
}
func (a SQLAuth) Register(ctx context.Context, email, password, display string) error {
	_, err := a.Users.Create(ctx, email, display, password)
	return err
}
func (a SQLAuth) Login(ctx context.Context, email, password string) (*User, error) {
	u, err := a.Users.ByEmail(ctx, email)
	if err != nil {
		return nil, ErrUnauthorized
	}
	ok, err := a.Users.CheckPassword(ctx, u, password)
	if err != nil || !ok {
		return nil, ErrUnauthorized
	}
	return &User{ID: u.ID.String(), Email: u.Email, DisplayName: u.DisplayName}, nil
}
func (a SQLAuth) Logout(ctx context.Context, r *http.Request) error {
	s, err := a.Sessions.Current(ctx, r)
	if err != nil {
		return nil
	}
	return a.Sessions.Store.DeleteSession(ctx, s.TokenHash)
}
func (a SQLAuth) ValidateCSRF(r *http.Request) error {
	s, err := a.Sessions.Current(r.Context(), r)
	if err != nil {
		return err
	}
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.FormValue("csrfToken")
	}
	if !a.Sessions.CheckCSRF(s, token) {
		return ErrForbidden
	}
	return nil
}

func (a SQLAuth) StartSession(ctx context.Context, w http.ResponseWriter, u *User) error {
	id, err := uuid.Parse(u.ID)
	if err != nil {
		return err
	}
	_, token, csrf, err := a.Sessions.New(ctx, id)
	if err != nil {
		return err
	}
	a.Sessions.SetCookie(w, token)
	http.SetCookie(w, &http.Cookie{Name: "santa_go_csrf", Value: csrf, Path: "/", Secure: a.Sessions.Secure, SameSite: http.SameSiteStrictMode, MaxAge: int(a.Sessions.TTL.Seconds())})
	return nil
}
