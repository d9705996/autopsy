// Package handler contains HTTP handlers grouped by resource.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/d9705996/autopsy/internal/api/jsonapi"
	"github.com/d9705996/autopsy/internal/auth"
	"github.com/d9705996/autopsy/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler handles /api/v1/auth/* routes.
type AuthHandler struct {
	db         *gorm.DB
	refresh    *auth.RefreshStore
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(db *gorm.DB, jwtSecret string, accessTTL, refreshTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		db:         db,
		refresh:    auth.NewRefreshStore(db),
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"` //nolint:gosec // intentional: login request DTO carries the user-supplied password
}

type tokenAttrs struct {
	AccessToken  string `json:"access_token"`  //nolint:gosec // intentional: auth response DTO carrying the issued access token
	RefreshToken string `json:"refresh_token"` //nolint:gosec // intentional: auth response DTO carrying the issued refresh token
	TokenType    string `json:"token_type"`
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonapi.RenderError(w, http.StatusBadRequest, "invalid_body", "Bad Request", "request body must be valid JSON")
		return
	}
	if req.Email == "" || req.Password == "" {
		jsonapi.RenderError(w, http.StatusUnprocessableEntity, "missing_field", "Unprocessable Entity", "email and password are required")
		return
	}

	ctx := r.Context()

	var u model.User
	if err := h.db.WithContext(ctx).
		Where("email = ? AND deactivated_at IS NULL", req.Email).
		First(&u).Error; err != nil {
		jsonapi.RenderError(w, http.StatusUnauthorized, "invalid_credentials", "Unauthorized", "email or password is incorrect")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		jsonapi.RenderError(w, http.StatusUnauthorized, "invalid_credentials", "Unauthorized", "email or password is incorrect")
		return
	}

	orgIDStr := ""
	if u.OrganizationID != nil {
		orgIDStr = *u.OrganizationID
	}

	accessToken, err := auth.IssueAccessToken(u.ID, u.Email, []string(u.Roles), orgIDStr, h.jwtSecret, h.accessTTL)
	if err != nil {
		jsonapi.RenderError(w, http.StatusInternalServerError, "token_error", "Internal Server Error", "failed to issue access token")
		return
	}

	refreshToken, err := h.refresh.IssueRefreshToken(ctx, u.ID, h.refreshTTL)
	if err != nil {
		jsonapi.RenderError(w, http.StatusInternalServerError, "token_error", "Internal Server Error", "failed to issue refresh token")
		return
	}

	jsonapi.RenderOne(w, http.StatusOK, jsonapi.ResourceObject{
		Type: "auth_token",
		ID:   u.ID,
		Attributes: tokenAttrs{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			TokenType:    "Bearer",
		},
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"` //nolint:gosec // intentional: token rotation/logout DTO
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		jsonapi.RenderError(w, http.StatusBadRequest, "invalid_body", "Bad Request", "refresh_token is required")
		return
	}

	ctx := r.Context()
	newRefresh, userID, err := h.refresh.RotateRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		jsonapi.RenderError(w, http.StatusUnauthorized, "invalid_token", "Unauthorized", "refresh token is invalid or expired")
		return
	}

	var u model.User
	if err := h.db.WithContext(ctx).
		Where("id = ? AND deactivated_at IS NULL", userID).
		First(&u).Error; err != nil {
		jsonapi.RenderError(w, http.StatusUnauthorized, "user_not_found", "Unauthorized", "user account does not exist")
		return
	}

	orgIDStr := ""
	if u.OrganizationID != nil {
		orgIDStr = *u.OrganizationID
	}

	accessToken, err := auth.IssueAccessToken(u.ID, u.Email, []string(u.Roles), orgIDStr, h.jwtSecret, h.accessTTL)
	if err != nil {
		jsonapi.RenderError(w, http.StatusInternalServerError, "token_error", "Internal Server Error", "failed to issue access token")
		return
	}

	jsonapi.RenderOne(w, http.StatusOK, jsonapi.ResourceObject{
		Type: "auth_token",
		ID:   u.ID,
		Attributes: tokenAttrs{
			AccessToken:  accessToken,
			RefreshToken: newRefresh,
			TokenType:    "Bearer",
		},
	})
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"` //nolint:gosec // intentional: token rotation/logout DTO
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		jsonapi.RenderError(w, http.StatusBadRequest, "invalid_body", "Bad Request", "refresh_token is required")
		return
	}
	// Ignore error: even if token not found, return 204 to avoid token probing.
	_ = h.refresh.RevokeRefreshToken(r.Context(), req.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}
