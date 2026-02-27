package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/d9705996/autopsy/internal/model"
	"gorm.io/gorm"
)

// RefreshStore manages refresh token persistence via GORM.
type RefreshStore struct {
	db *gorm.DB
}

// NewRefreshStore creates a RefreshStore backed by the given GORM DB.
func NewRefreshStore(db *gorm.DB) *RefreshStore {
	return &RefreshStore{db: db}
}

// IssueRefreshToken generates a secure random token, stores its SHA-256 hash,
// and returns the plaintext token to the caller (stored nowhere).
func (s *RefreshStore) IssueRefreshToken(_ context.Context, userID string, ttl time.Duration) (string, error) {
	raw, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	h := hashToken(raw)

	rt := &model.RefreshToken{
		UserID:    userID,
		TokenHash: h,
		ExpiresAt: time.Now().Add(ttl),
	}
	if err := s.db.Create(rt).Error; err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return raw, nil
}

// RotateRefreshToken validates the given token, revokes it, and issues a new one.
// Returns the new refresh token and the user ID.
func (s *RefreshStore) RotateRefreshToken(_ context.Context, rawToken string) (token string, userID string, err error) {
	h := hashToken(rawToken)

	var rt model.RefreshToken
	if err := s.db.Where("token_hash = ?", h).First(&rt).Error; err != nil {
		return "", "", fmt.Errorf("refresh token not found: %w", err)
	}
	if rt.RevokedAt != nil {
		return "", "", fmt.Errorf("refresh token has been revoked")
	}
	if time.Now().After(rt.ExpiresAt) {
		return "", "", fmt.Errorf("refresh token has expired")
	}

	// Revoke the old token.
	now := time.Now()
	if err := s.db.Model(&rt).Update("revoked_at", now).Error; err != nil {
		return "", "", fmt.Errorf("revoke old refresh token: %w", err)
	}

	// Issue new token.
	newRaw, err := generateToken()
	if err != nil {
		return "", "", fmt.Errorf("generate new refresh token: %w", err)
	}
	newRT := &model.RefreshToken{
		UserID:    rt.UserID,
		TokenHash: hashToken(newRaw),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	if err := s.db.Create(newRT).Error; err != nil {
		return "", "", fmt.Errorf("store new refresh token: %w", err)
	}

	return newRaw, rt.UserID, nil
}

// RevokeRefreshToken marks the given token as revoked.
func (s *RefreshStore) RevokeRefreshToken(_ context.Context, rawToken string) error {
	h := hashToken(rawToken)
	return s.db.Model(&model.RefreshToken{}).
		Where("token_hash = ?", h).
		Update("revoked_at", time.Now()).Error
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
