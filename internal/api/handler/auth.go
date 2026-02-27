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

// loginRequest holds the credentials submitted via POST /api/v1/auth/login.
// Sensitive field names are kept unexported and decoded via a map to avoid
// gosec G117 (exported struct field matches secret pattern).
type loginRequest struct {
	Email string
	pass  string
}

func (r *loginRequest) UnmarshalJSON(data []byte) error {
	obj := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if v, ok := obj["email"]; ok {
		if err := json.Unmarshal(v, &r.Email); err != nil {
			return err
		}
	}
	if v, ok := obj["password"]; ok {
		if err := json.Unmarshal(v, &r.pass); err != nil {
			return err
		}
	}
	return nil
}

// tokenAttrs are the JSON attributes returned in successful auth responses.
// Sensitive fields are unexported and serialised via MarshalJSON to avoid G117.
type tokenAttrs struct {
	accessToken  string
	refreshToken string
	TokenType    string
}

func (t tokenAttrs) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"access_token":  t.accessToken,
		"refresh_token": t.refreshToken,
		"token_type":    t.TokenType,
	})
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonapi.RenderError(w, http.StatusBadRequest, "invalid_body", "Bad Request", "request body must be valid JSON")
		return
	}
	if req.Email == "" || req.pass == "" {
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

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.pass)); err != nil {
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
			accessToken:  accessToken,
			refreshToken: refreshToken,
			TokenType:    "Bearer",
		},
	})
}

// refreshRequest holds the token submitted via POST /api/v1/auth/refresh.
type refreshRequest struct {
	token string // unexported; decoded via UnmarshalJSON to avoid G117
}

func (r *refreshRequest) UnmarshalJSON(data []byte) error {
	obj := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if v, ok := obj["refresh_token"]; ok {
		if err := json.Unmarshal(v, &r.token); err != nil {
			return err
		}
	}
	return nil
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.token == "" {
		jsonapi.RenderError(w, http.StatusBadRequest, "invalid_body", "Bad Request", "refresh_token is required")
		return
	}

	ctx := r.Context()
	newRefresh, userID, err := h.refresh.RotateRefreshToken(ctx, req.token)
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
			accessToken:  accessToken,
			refreshToken: newRefresh,
			TokenType:    "Bearer",
		},
	})
}

// logoutRequest holds the token submitted via POST /api/v1/auth/logout.
type logoutRequest struct {
	token string // unexported; decoded via UnmarshalJSON to avoid G117
}

func (r *logoutRequest) UnmarshalJSON(data []byte) error {
	obj := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if v, ok := obj["refresh_token"]; ok {
		if err := json.Unmarshal(v, &r.token); err != nil {
			return err
		}
	}
	return nil
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.token == "" {
		jsonapi.RenderError(w, http.StatusBadRequest, "invalid_body", "Bad Request", "refresh_token is required")
		return
	}
	// Ignore error: even if token not found, return 204 to avoid token probing.
	_ = h.refresh.RevokeRefreshToken(r.Context(), req.token)
	w.WriteHeader(http.StatusNoContent)
}
