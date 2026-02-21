package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"time"
)

const cookieName = "autopsy_session"

type Auth struct {
	username string
	password string
}

func New(username, password string) *Auth {
	return &Auth{username: username, password: password}
}

func (a *Auth) Login(username, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(a.username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1
	return userOK && passOK
}

func (a *Auth) SetSession(w http.ResponseWriter, username string) {
	value := base64.StdEncoding.EncodeToString([]byte(username + "|ok"))
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(12 * time.Hour),
	})
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil || c.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
