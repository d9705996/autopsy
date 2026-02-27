// Package auth provides JWT token issuance and validation.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the set of custom claims stored inside a Autopsy access token.
type Claims struct {
	UserID         string   `json:"uid"`
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
	OrganizationID string   `json:"org_id,omitempty"`
	jwt.RegisteredClaims
}

// IssueAccessToken creates and signs a new JWT access token.
func IssueAccessToken(userID, email string, roles []string, orgID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:         userID,
		Email:          email,
		Roles:          roles,
		OrganizationID: orgID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "autopsy",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseAccessToken validates the token string and returns its Claims.
// Returns an error if the token is invalid, expired, or signed with a different key.
func ParseAccessToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
