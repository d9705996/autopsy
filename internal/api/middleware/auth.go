// Package middleware provides HTTP middleware for Autopsy.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/d9705996/autopsy/internal/api/jsonapi"
	"github.com/d9705996/autopsy/internal/auth"
)

type contextKey string

const claimsKey contextKey = "auth_claims"

// RequireAuth validates the Bearer JWT in the Authorization header.
// On success it injects *auth.Claims into the request context.
// On failure it writes a 401 JSON:API error response.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				jsonapi.RenderError(w, http.StatusUnauthorized,
					"missing_token", "Unauthorized", "Authorization header is required")
				return
			}

			claims, err := auth.ParseAccessToken(token, secret)
			if err != nil {
				jsonapi.RenderError(w, http.StatusUnauthorized,
					"invalid_token", "Unauthorized", "access token is invalid or expired")
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts Claims from the request context.
// Returns nil if not present.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	v := ctx.Value(claimsKey)
	if v == nil {
		return nil
	}
	c, _ := v.(*auth.Claims)
	return c
}

// RequirePermission checks that the authenticated user's roles grant the
// given permission string. Must be chained after RequireAuth.
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				jsonapi.RenderError(w, http.StatusUnauthorized,
					"missing_token", "Unauthorized", "authentication required")
				return
			}
			if !hasPermission(claims.Roles, perm) {
				jsonapi.RenderError(w, http.StatusForbidden,
					"forbidden", "Forbidden",
					"your roles do not grant the '"+perm+"' permission")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// rolePermissions maps built-in role names to their allowed permission strings.
// This will grow as endpoints are added (T048 for full RBAC).
var rolePermissions = map[string][]string{
	"Viewer": {
		"health:read",
		"alert:read",
		"incident:read",
		"postmortem:read",
		"slo:read",
		"oncall:read",
	},
	"Responder": {
		"health:read",
		"alert:read",
		"incident:read", "incident:create", "incident:update", "incident:comment",
		"postmortem:read",
		"slo:read",
		"oncall:read",
		"oncall:update",
	},
	"IncidentCommander": {
		"health:read",
		"alert:read",
		"incident:read", "incident:create", "incident:update", "incident:reopen", "incident:comment",
		"postmortem:read", "postmortem:update", "postmortem:publish",
		"slo:read",
		"oncall:read", "oncall:update",
		"action_item:read", "action_item:update",
	},
	"Admin": {"*"}, // wildcard — grants all permissions
}

func hasPermission(roles []string, perm string) bool {
	for _, role := range roles {
		perms := rolePermissions[role]
		for _, p := range perms {
			if p == "*" || p == perm {
				return true
			}
		}
	}
	return false
}
