package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

const cookieName = "autopsy_session"

type contextKey string

const userContextKey contextKey = "user"

type SessionUser struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

type Auth struct {
	secret []byte
}

func New(secret string) *Auth {
	if secret == "" {
		secret = "autopsy-dev-secret"
	}
	return &Auth{secret: []byte(secret)}
}

func (a *Auth) SetSession(w http.ResponseWriter, username string, roles []string) {
	payload := username + "|" + strings.Join(roles, ",")
	sig := a.sign(payload)
	value := base64.StdEncoding.EncodeToString([]byte(payload + "|" + sig))
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(12 * time.Hour)})
}

func (a *Auth) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Expires: time.Unix(0, 0)})
}

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

func (a *Auth) RequirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if hasPermission(user.Roles, permission) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

func UserFromContext(ctx context.Context) (SessionUser, bool) {
	u, ok := ctx.Value(userContextKey).(SessionUser)
	return u, ok
}

func hasPermission(roles []string, permission string) bool {
	for _, role := range roles {
		if role == "admin" || role == "*" || role == permission {
			return true
		}
	}
	return false
}

func (a *Auth) UserFromRequest(r *http.Request) (SessionUser, error) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return SessionUser{}, errors.New("missing session")
	}
	decoded, err := base64.StdEncoding.DecodeString(c.Value)
	if err != nil {
		return SessionUser{}, err
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) < 3 {
		return SessionUser{}, errors.New("invalid session")
	}
	payload := strings.Join(parts[:len(parts)-1], "|")
	if a.sign(payload) != parts[len(parts)-1] {
		return SessionUser{}, errors.New("invalid signature")
	}
	roles := []string{}
	if parts[1] != "" {
		roles = strings.Split(parts[1], ",")
	}
	return SessionUser{Username: parts[0], Roles: roles}, nil
}

func (a *Auth) sign(payload string) string {
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
