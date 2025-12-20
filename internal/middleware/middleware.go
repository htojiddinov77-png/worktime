package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const userIDContextKey contextKey = "userID"

type Middleware struct {
}

func GetUserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(userIDContextKey).(int64)
	return userID,ok
}


func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1) Read Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing auth header", http.StatusUnauthorized)
			return
		}

		// 2) Parse and validate JWT (pseudo-code)
		// token, err := jwt.Parse(...)
		// userID := token.Claims["user_id"]

		userID := int64(42) // pretend extracted from JWT

		// 3) Store userID in context
		ctx := context.WithValue(r.Context(), userIDContextKey, userID)

		// 4) Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
