package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	cookieName    = "autopsy_session"
	sessionMaxAge = 12 * time.Hour
)

var (
	errMissingSession = errors.New("missing session")
	errInvalidSession = errors.New("invalid session")
)

type contextKey string

const userContextKey contextKey = "user"

// SessionUser is the validated user stored in the request context.
// Permissions holds the flattened set of permission strings from the
// user's roles (e.g. ["*"] for admin, ["read:dashboard"] for viewer).
type SessionUser struct {
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
}

// claims is the JWT payload.
type claims struct {
	Permissions []string `json:"perms"`
	jwt.RegisteredClaims
}

// Auth handles JWT session creation and validation.
type Auth struct {
	secret []byte
}

// New returns an Auth instance backed by the provided HMAC secret.
func New(secret string) *Auth {
	return &Auth{secret: []byte(secret)}
}

// SetSession writes a signed JWT into an HttpOnly cookie. The token
// carries the flattened list of permissions so no DB round-trip is
// needed on every authenticated request.
func (a *Auth) SetSession(w http.ResponseWriter, username string, permissions []string) {
	now := time.Now()
	c := &claims{
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(sessionMaxAge)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := token.SignedString(a.secret)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
}

// ClearSession removes the session cookie.
func (a *Auth) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// Middleware validates the JWT and injects the SessionUser into the
// request context. Unauthenticated requests receive 401.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := a.UserFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

// RequirePermission is applied after Middleware and checks that the
// authenticated user holds the requested permission.
func (a *Auth) RequirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if hasPermission(user.Permissions, permission) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

// UserFromContext retrieves the authenticated user from the context.
func UserFromContext(ctx context.Context) (SessionUser, bool) {
	u, ok := ctx.Value(userContextKey).(SessionUser)
	return u, ok
}

// hasPermission returns true if any entry in perms is "*" or matches
// the required permission string exactly.
func hasPermission(perms []string, permission string) bool {
	for _, p := range perms {
		if p == "*" || p == permission {
			return true
		}
	}
	return false
}

// UserFromRequest parses and validates the JWT session cookie.
func (a *Auth) UserFromRequest(r *http.Request) (SessionUser, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return SessionUser{}, errMissingSession
	}

	var c claims
	token, err := jwt.ParseWithClaims(cookie.Value, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errInvalidSession
		}
		return a.secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return SessionUser{}, errInvalidSession
	}

	return SessionUser{Username: c.Subject, Permissions: c.Permissions}, nil
}
