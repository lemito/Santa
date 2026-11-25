package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	if s.Static != nil {
		mux.Handle("/static/", http.StripPrefix("/static/", s.Static))
	}
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/health/ready", s.ready)
	mux.HandleFunc("/api/session", s.session)
	mux.HandleFunc("/account/register", s.register)
	mux.HandleFunc("/account/login", s.login)
	mux.HandleFunc("/account/logout", s.logout)
	mux.HandleFunc("/api/games/", s.games)
	mux.HandleFunc("/api/games", s.games)
	mux.HandleFunc("/actions/create", s.actionCreate)
	mux.HandleFunc("/actions/", s.actions)
	mux.HandleFunc("/", s.page)
	limit := s.BodyLimit
	if limit <= 0 {
		limit = 1 << 20
	}
	return withBodyLimit(limit)(withRequestID(s.Logger, mux))
}
func withBodyLimit(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

type requestIDContextKey struct{}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func withRequestID(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Santa-Backend", "go")
		requestID := r.Header.Get("X-Request-ID")
		if !safeRequestID(requestID) {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if logger != nil {
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.Info("http_request",
				zap.String("request_id", requestID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", status),
				zap.Int("bytes", recorder.bytes),
				zap.Duration("duration", time.Since(started)))
		}
	})
}
func safeRequestID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Ready != nil && !s.Ready(r.Context()) {
		writeJSON(w, 503, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil {
		writeJSON(w, 200, map[string]any{"authenticated": false})
		return
	}
	sess, err := s.Auth.Session(r)
	if err != nil || sess == nil || sess.User == nil {
		writeJSON(w, 200, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, 200, map[string]any{"authenticated": true, "user": sess.User, "csrfToken": sess.CSRF})
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if !s.allow(r) {
		writeError(w, 429, "rate_limited")
		return
	}
	if !sameOrigin(r) {
		writeError(w, 403, "csrf")
		return
	}
	email := normalizeEmail(r.FormValue("email"))
	pw := r.FormValue("password")
	display := strings.TrimSpace(r.FormValue("displayName"))
	if err := s.Auth.Register(r.Context(), email, pw, display); err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}
	if u, err := s.Auth.Login(r.Context(), email, pw); err == nil {
		if starter, ok := s.Auth.(interface {
			StartSession(context.Context, http.ResponseWriter, *User) error
		}); ok {
			_ = starter.StartSession(r.Context(), w, u)
		}
	}
	if wantsJSON(r) {
		writeJSON(w, 201, map[string]string{"status": "registered"})
	} else {
		http.Redirect(w, r, "/", 303)
	}
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if !s.allow(r) {
		writeError(w, 429, "rate_limited")
		return
	}
	if !sameOrigin(r) {
		writeError(w, 403, "csrf")
		return
	}
	u, err := s.Auth.Login(r.Context(), normalizeEmail(r.FormValue("email")), r.FormValue("password"))
	if err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}
	if starter, ok := s.Auth.(interface {
		StartSession(context.Context, http.ResponseWriter, *User) error
	}); ok {
		if err := starter.StartSession(r.Context(), w, u); err != nil {
			writeError(w, 500, "session_failed")
			return
		}
	}
	if wantsJSON(r) {
		writeJSON(w, 200, map[string]any{"status": "ok", "user": u})
	} else {
		http.Redirect(w, r, "/", 303)
	}
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if err := s.requireCSRF(r); err != nil {
		writeError(w, 403, "csrf")
		return
	}
	if err := s.Auth.Logout(r.Context(), r); err != nil {
		writeError(w, 500, "logout_failed")
		return
	}
	secure := false
	if provider, ok := s.Auth.(interface{ SecureCookies() bool }); ok {
		secure = provider.SecureCookies()
	}
	clearSessionCookies(w, secure)
	if wantsJSON(r) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	} else {
		http.Redirect(w, r, "/", 303)
	}
}
func clearSessionCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{"santa_go_session", "santa_go_csrf"} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: name == "santa_go_session", Secure: secure, SameSite: http.SameSiteStrictMode})
	}
}
func (s *Server) publish(ctx context.Context, u *User, id string, in PublishRequest) error {
	if s.DrawGate != nil {
		s.DrawGate <- struct{}{}
		defer func() { <-s.DrawGate }()
	}
	return s.Games.Publish(ctx, u, id, in)
}
func (s *Server) games(w http.ResponseWriter, r *http.Request) {
	u, ok := s.user(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/games")
	if path == "" || path == "/" {
		if r.Method == "GET" {
			v, e := s.Games.List(r.Context(), u)
			if e != nil {
				writeErr(w, e)
				return
			}
			writeJSON(w, 200, v)
			return
		}
		if r.Method == "POST" {
			if err := s.requireCSRF(r); err != nil {
				writeError(w, 403, "csrf")
				return
			}
			var in struct {
				Title string `json:"title"`
			}
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				writeError(w, 422, "invalid_json")
				return
			}
			id, e := s.Games.Create(r.Context(), u, strings.TrimSpace(in.Title))
			if e != nil {
				writeErr(w, e)
				return
			}
			writeJSON(w, 201, map[string]string{"id": id})
			return
		}
		w.WriteHeader(405)
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	id := parts[0]
	if len(parts) == 1 && r.Method == "GET" {
		v, e := s.Games.Get(r.Context(), u, id)
		if e != nil {
			writeErr(w, e)
			return
		}
		writeJSON(w, 200, v)
		return
	}
	if len(parts) == 2 && parts[1] == "configuration" {
		if r.Method == "GET" {
			v, e := s.Games.Configuration(r.Context(), u, id)
			if e != nil {
				writeErr(w, e)
				return
			}
			writeJSON(w, 200, v)
			return
		}
		if r.Method == "PUT" {
			if err := s.requireCSRF(r); err != nil {
				writeError(w, 403, "csrf")
				return
			}
			var in Configuration
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				writeError(w, 422, "invalid_json")
				return
			}
			if err := s.Games.Configure(r.Context(), u, id, in, in.Version); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]string{"status": "ok"})
			return
		}
	}
	if len(parts) == 2 && (parts[1] == "my-message" || parts[1] == "my-recipient-message" || parts[1] == "message-from-santa") {
		if r.Method == http.MethodGet {
			var v Message
			var e error
			switch parts[1] {
			case "my-message":
				v, e = s.Games.MyMessage(r.Context(), u, id)
			case "my-recipient-message":
				v, e = s.Games.MyRecipientMessage(r.Context(), u, id)
			case "message-from-santa":
				v, e = s.Games.MessageFromSanta(r.Context(), u, id)
			}
			if e != nil {
				writeErr(w, e)
				return
			}
			writeJSON(w, 200, v)
			return
		}
		if r.Method == http.MethodPut && parts[1] != "message-from-santa" {
			if err := s.requireCSRF(r); err != nil {
				writeError(w, 403, "csrf")
				return
			}
			var in struct {
				Message string `json:"message"`
			}
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				writeError(w, 422, "invalid_json")
				return
			}
			var e error
			if parts[1] == "my-message" {
				e = s.Games.SetMessage(r.Context(), u, id, in.Message)
			} else {
				e = s.Games.SetRecipientMessage(r.Context(), u, id, in.Message)
			}
			if e != nil {
				writeErr(w, e)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	if len(parts) == 2 && (parts[1] == "publish" || parts[1] == "close" || parts[1] == "my-assignment") {
		switch parts[1] {
		case "publish":
			if r.Method != "POST" {
				break
			}
			if err := s.requireCSRF(r); err != nil {
				writeError(w, 403, "csrf")
				return
			}
			var in PublishRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			if err := s.publish(r.Context(), u, id, in); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]string{"status": "published"})
			return
		case "close":
			if r.Method != "POST" {
				break
			}
			if err := s.requireCSRF(r); err != nil {
				writeError(w, 403, "csrf")
				return
			}
			if err := s.Games.Close(r.Context(), u, id); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]string{"status": "closed"})
			return
		case "my-assignment":
			if r.Method != "GET" {
				break
			}
			v, e := s.Games.Assignment(r.Context(), u, id)
			if e != nil {
				writeErr(w, e)
				return
			}
			writeJSON(w, 200, v)
			return
		}
	}
	writeError(w, 404, "not_found")
}
func (s *Server) actionCreate(w http.ResponseWriter, r *http.Request) {
	u, ok := s.user(w, r)
	if !ok {
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if err := s.requireCSRF(r); err != nil {
		redirectErr(w, r, "csrf")
		return
	}
	id, e := s.Games.Create(r.Context(), u, strings.TrimSpace(r.FormValue("title")))
	if e != nil {
		redirectErr(w, r, e.Error())
		return
	}
	http.Redirect(w, r, "/games/"+id, 303)
}
func (s *Server) actions(w http.ResponseWriter, r *http.Request) {
	u, ok := s.user(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/actions/"), "/"), "/")
	if len(parts) != 2 {
		writeError(w, 404, "not_found")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.requireCSRF(r); err != nil {
		redirectErr(w, r, "csrf")
		return
	}
	id, op := parts[0], parts[1]
	switch op {
	case "publish":
		var in PublishRequest
		in.RequestKey = r.FormValue("requestKey")
		fmt.Sscan(r.FormValue("version"), &in.Version)
		if e := s.publish(r.Context(), u, id, in); e != nil {
			redirectErr(w, r, e.Error())
			return
		}
	case "close":
		if e := s.Games.Close(r.Context(), u, id); e != nil {
			redirectErr(w, r, e.Error())
			return
		}
	case "configure":
		cfg := Configuration{}
		for _, email := range strings.Split(r.FormValue("emails"), "\n") {
			email = normalizeEmail(email)
			if email != "" {
				cfg.Participants = append(cfg.Participants, Participant{Email: email})
			}
		}
		var err error
		cfg.Exclusions, err = parseExclusions(r.FormValue("exclusions"))
		if err != nil {
			redirectErr(w, r, err.Error())
			return
		}

		var ver int64
		fmt.Sscan(r.FormValue("version"), &ver)
		if e := s.Games.Configure(r.Context(), u, id, cfg, ver); e != nil {
			redirectErr(w, r, e.Error())
			return
		}
	case "message":
		if e := s.Games.SetMessage(r.Context(), u, id, r.FormValue("message")); e != nil {
			redirectErr(w, r, e.Error())
			return
		}
	case "recipient-message":
		if e := s.Games.SetRecipientMessage(r.Context(), u, id, r.FormValue("message")); e != nil {
			redirectErr(w, r, e.Error())
			return
		}
	default:
		redirectErr(w, r, "not_found")
		return
	}
	http.Redirect(w, r, "/games/"+id, 303)
}
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/login" && r.URL.Path != "/register" && r.URL.Path != "/not-found" && r.URL.Path != "/Error" && !strings.HasPrefix(r.URL.Path, "/games/") {
		http.NotFound(w, r)
		return
	}
	if s.Templates == nil {
		w.Write([]byte("<html><body><h1>Santa</h1></body></html>"))
		return
	}
	name := "home"
	data := map[string]any{"Path": r.URL.Path}
	if r.URL.Path == "/login" {
		name = "login"
	} else if r.URL.Path == "/register" {
		name = "register"
	} else if r.URL.Path == "/not-found" || r.URL.Path == "/Error" {
		name = "not-found"
	} else if strings.HasPrefix(r.URL.Path, "/games/") {
		name = "game"
		u, ok := s.user(w, r)
		if !ok {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/games/")
		g, err := s.Games.Get(r.Context(), u, id)
		if err != nil {
			writeErr(w, err)
			return
		}
		data["Game"] = g
		data["Title"], data["State"] = g.Title, g.State
		data["RequestKey"] = uuid.NewString()
		if g.Owner && g.State == "Draft" {
			if cfg, e := s.Games.Configuration(r.Context(), u, id); e == nil {
				var emails, exclusions strings.Builder
				for _, p := range cfg.Participants {
					emails.WriteString(p.Email)
					emails.WriteByte('\n')
				}
				for _, p := range cfg.Exclusions {
					exclusions.WriteString(p.First)
					exclusions.WriteString(", ")
					exclusions.WriteString(p.Second)
					exclusions.WriteByte('\n')
				}
				data["Configuration"], data["Emails"], data["Exclusions"] = cfg, emails.String(), exclusions.String()
			}
		}
		if g.State != "Closed" {
			if m, e := s.Games.MyMessage(r.Context(), u, id); e == nil {
				data["MessageToSanta"], data["CanMessageSanta"] = m, true
			}
		}
		if g.State == "Published" {
			if v, e := s.Games.Assignment(r.Context(), u, id); e == nil {
				data["Recipient"], data["HasRecipient"] = v, true
			}
			if m, e := s.Games.MyRecipientMessage(r.Context(), u, id); e == nil {
				data["MessageToRecipient"], data["CanMessageRecipient"] = m, true
			}
			if m, e := s.Games.MessageFromSanta(r.Context(), u, id); e == nil && m.HTML != "" {
				data["MessageFromSanta"], data["HasMessageFromSanta"] = m, true
			}
		}
	}
	csrf := ""
	if s.Auth != nil {
		if session, _ := s.Auth.Session(r); session != nil {
			csrf = session.CSRF
			data["Authenticated"], data["User"] = session.User != nil, session.User
			if name == "home" && session.User != nil {
				if games, e := s.Games.List(r.Context(), session.User); e == nil {
					data["Games"] = games
				}
			}
		}
	}
	data["CSRF"], data["Page"] = csrf, name
	_ = s.Templates.T.ExecuteTemplate(w, "layout", data)
}
func (s *Server) user(w http.ResponseWriter, r *http.Request) (*User, bool) {
	if s.Auth == nil {
		writeError(w, 503, "auth_unavailable")
		return nil, false
	}
	sess, e := s.Auth.Session(r)
	if e != nil || sess == nil || sess.User == nil {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, 401, "unauthorized")
		} else {
			http.Redirect(w, r, "/login", 303)
		}
		return nil, false
	}
	return sess.User, true
}
func (s *Server) requireCSRF(r *http.Request) error {
	if s.Auth == nil {
		return ErrUnauthorized
	}
	return s.Auth.ValidateCSRF(r)
}
func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	return err == nil && u.Host == r.Host
}
func (s *Server) allow(r *http.Request) bool {
	if s.Rate == nil {
		return true
	}
	key := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		key = host
	}
	return s.Rate.Allow(key)
}
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") || strings.HasPrefix(r.URL.Path, "/api/")
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func writeErr(w http.ResponseWriter, e error) {
	status := statusFor(e)
	msg := "internal_error"
	if status != 500 {
		msg = e.Error()
	}
	writeError(w, status, msg)
}
func statusFor(e error) int {
	if errors.Is(e, ErrUnauthorized) {
		return 401
	}
	if errors.Is(e, ErrForbidden) {
		return 403
	}
	if errors.Is(e, ErrNotFound) {
		return 404
	}
	if errors.Is(e, ErrConflict) {
		return 409
	}
	if errors.Is(e, ErrValidation) {
		return 422
	}
	return 500
}
func redirectErr(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/?error="+template.URLQueryEscaper(msg), 303)
}
