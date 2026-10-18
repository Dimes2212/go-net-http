package auth

import (
	"context"
	"net/http"
)

type contextKey struct{}

var userIDKey contextKey

func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Error(
				w,
				"unathorized",
				http.StatusUnauthorized,
			)
			return
		}

		userId, err := s.ValidateSession(cookie.Value)
		if err != nil {
			http.Error(
				w,
				"unathorized",
				http.StatusUnauthorized,
			)
			return
		}

		ctx := context.WithValue(
			r.Context(),
			userIDKey,
			userId,
		)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	userId, ok := ctx.Value(userIDKey).(int64)
	return userId, ok
}
