package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/htojiddinov77-png/worktime/internal/auth"
)

type contextKey string

const (
	userIDContextKey   contextKey = "userID"
	userRoleContextKey contextKey = "userRole"
	// userEmailContextKey contextKey = "userEmail" // optional
)

type Middleware struct {
	JWT *auth.JWTManager
}

func GetUserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(userIDContextKey).(int64)
	return userID, ok
}

func GetUserRole(r *http.Request) (string, bool) {
	role, ok := r.Context().Value(userRoleContextKey).(string)
	return role, ok
}

// func GetUserEmail(r *http.Request) (string, bool) { ... } // optional

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || m.JWT == nil {
			http.Error(w, "server misconfigured", http.StatusInternalServerError)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing auth header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := m.JWT.VerifyToken(tokenString)
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Store auth info in context
		ctx := context.WithValue(r.Context(), userIDContextKey, claims.UserID)
		ctx = context.WithValue(ctx, userRoleContextKey, claims.Role)
		// ctx = context.WithValue(ctx, userEmailContextKey, claims.Email) // optional

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := GetUserRole(r)
		if !ok || role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func CORS(allowedOrigins map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			origin := r.Header.Get("Origin")
			if allowedOrigins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
