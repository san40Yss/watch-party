// Package auth is the identity seam: users, sessions, and a middleware that
// attaches the current user to the request context. It is intentionally
// minimal — there is no registration/login UI yet — but the seam means adding
// real auth later (Step 5, rooms) is localized rather than a rewrite.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	minUsernameLen = 3
	minPasswordLen = 6
)

const (
	cookieName      = "session"
	sessionTTL      = 30 * 24 * time.Hour
	defaultUser     = "host"
	defaultPassword = "changeme"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

type ctxKey int

const userKey ctxKey = 0

// WithUser stores the current user in the context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFrom returns the current user, or nil if the request is anonymous.
func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}

type Service struct {
	pool          *pgxpool.Pool
	devAutoLogin  bool // when true, anonymous requests fall back to the default user
	defaultUserID int
}

func New(pool *pgxpool.Pool, devAutoLogin bool) *Service {
	return &Service{pool: pool, devAutoLogin: devAutoLogin}
}

// EnsureDefaultUser creates the seed user if the table is empty and backfills
// ownership on any pre-existing videos. Caches the default user id for the
// dev auto-login fallback.
func (s *Service) EnsureDefaultUser(ctx context.Context) error {
	var id int
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE username = $1`, defaultUser).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		hash, herr := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
		if herr != nil {
			return herr
		}
		if err = s.pool.QueryRow(ctx,
			`INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`,
			defaultUser, string(hash)).Scan(&id); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	s.defaultUserID = id
	// The seed user administers the library.
	if _, err = s.pool.Exec(ctx,
		`UPDATE users SET is_admin = true WHERE id = $1`, id); err != nil {
		return err
	}
	// Backfill ownership for videos created before the auth seam existed.
	_, err = s.pool.Exec(ctx,
		`UPDATE videos SET owner_id = $1 WHERE owner_id IS NULL`, id)
	return err
}

// Middleware resolves the session cookie to a user and attaches it to the
// context. With dev auto-login on, anonymous requests get the default user so
// the app keeps working before a login UI exists.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := s.userFromRequest(r)
		if user == nil && s.devAutoLogin && s.defaultUserID != 0 {
			user = &User{ID: s.defaultUserID, Username: defaultUser, IsAdmin: true}
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

// RequireAuth rejects requests with no authenticated user (401). Wrap the
// protected routes with it; public routes (login, register, me) stay outside.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()) == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin rejects non-admins (403). Library mutation — processing,
// deletion, uploads — is admin-only; everyone else just watches.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		if u == nil || !u.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) userFromRequest(r *http.Request) *User {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	var u User
	err = s.pool.QueryRow(r.Context(),
		`SELECT u.id, u.username, u.is_admin FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token = $1 AND s.expires_at > NOW()`, c.Value).Scan(&u.ID, &u.Username, &u.IsAdmin)
	if err != nil {
		return nil
	}
	return &u
}

// Login verifies credentials, creates a session, and sets the cookie.
func (s *Service) Login(w http.ResponseWriter, r *http.Request, username, password string) (*User, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, username, password_hash, is_admin FROM users WHERE username = $1`,
		username).Scan(&u.ID, &u.Username, &hash, &u.IsAdmin)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, errors.New("invalid credentials")
	}
	if err := s.createSession(w, r, u.ID); err != nil {
		return nil, err
	}
	return &u, nil
}

// Register creates a new account and immediately logs it in. Usernames are
// unique (enforced by the DB); we surface the collision as a friendly error.
func (s *Service) Register(w http.ResponseWriter, r *http.Request, username, password string) (*User, error) {
	username = strings.TrimSpace(username)
	if len([]rune(username)) < minUsernameLen {
		return nil, errors.New("имя пользователя — минимум 3 символа")
	}
	if len(password) < minPasswordLen {
		return nil, errors.New("пароль — минимум 6 символов")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := User{Username: username}
	err = s.pool.QueryRow(r.Context(),
		`INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`,
		username, string(hash)).Scan(&u.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errors.New("имя пользователя занято")
		}
		return nil, err
	}

	if err := s.createSession(w, r, u.ID); err != nil {
		return nil, err
	}
	return &u, nil
}

// ChangePassword updates a user's password after verifying the current one.
// Existing sessions stay valid (the user changing it keeps their own).
func (s *Service) ChangePassword(ctx context.Context, userID int, current, newPass string) error {
	if len(newPass) < minPasswordLen {
		return errors.New("новый пароль — минимум 6 символов")
	}

	var hash string
	if err := s.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return errors.New("пользователь не найден")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		return errors.New("текущий пароль неверный")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`, string(newHash), userID)
	return err
}

// createSession issues a session token, stores it, and sets the cookie.
func (s *Service) createSession(w http.ResponseWriter, r *http.Request, userID int) error {
	token, err := newToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionTTL)
	if _, err := s.pool.Exec(r.Context(),
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, userID, expires); err != nil {
		return err
	}
	s.setCookie(w, token, expires)
	return nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Logout deletes the session and clears the cookie.
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		_, _ = s.pool.Exec(r.Context(), `DELETE FROM sessions WHERE token = $1`, c.Value)
	}
	s.setCookie(w, "", time.Unix(0, 0))
}

func (s *Service) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
