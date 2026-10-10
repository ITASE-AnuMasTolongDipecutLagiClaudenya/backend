package middleware

import (
	"context"
	"net/http"
	"strings"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/supabase"
)

type Identity struct{ ID, Email string }

type ctxKey struct{}

func FromContext(ctx context.Context) Identity {
	u, _ := ctx.Value(ctxKey{}).(Identity)
	return u
}

func Authenticate(sb *supabase.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			if token == "" {
				httpx.WriteError(w, 401, "Bearer token diperlukan")
				return
			}
			id, email, err := sb.ValidateUser(r.Context(), token)
			if err != nil {
				if strings.Contains(err.Error(), "invalid session") {
					httpx.WriteError(w, 401, "Sesi tidak valid atau kedaluwarsa")
					return
				}
				httpx.WriteError(w, 502, "Validasi sesi Supabase gagal")
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, Identity{ID: id, Email: email})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
