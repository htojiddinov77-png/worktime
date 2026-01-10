package middleware

import (
	"context"
	"net/http"

	"github.com/htojiddinov77-png/worktime/internal/auth"
	"github.com/htojiddinov77-png/worktime/internal/utils"
)

type contextKey string

const UserContextKey = contextKey("user")

type Middleware struct {
	JWT *auth.JWTManager
}

func SetUser(r *http.Request, user *auth.UserClaims) *http.Request {
	ctx := context.WithValue(r.Context(), UserContextKey, user)
	return r.WithContext(ctx)
}

func GetUser(r *http.Request) *auth.UserClaims {
	user, ok := r.Context().Value(UserContextKey).(*auth.UserClaims)
	if !ok {
		panic("missing user in request")
	}
	return user
}

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			r = SetUser(r, auth.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		tokenString, err := m.JWT.ExtractBearerToken(r)
		if err != nil {
			utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": err.Error()})
			return
		}

		claims, err := m.JWT.VerifyToken(tokenString)
		if err != nil {
			utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": err.Error()})
			return
		}

		r = SetUser(r, claims)
		next.ServeHTTP(w, r)
		return
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
