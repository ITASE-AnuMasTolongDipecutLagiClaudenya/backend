package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/supabase"
)

const idleTimeout = 2 * time.Hour

var (
	idleMu   sync.Mutex
	lastSeen = map[string]time.Time{}
)

// ponytail: process-local, entries never swept (single instance, small app).
// Move to Redis/DB if the API ever runs multi-instance.
func active(key string) bool {
	idleMu.Lock()
	defer idleMu.Unlock()
	now := time.Now()
	if last, ok := lastSeen[key]; ok && now.Sub(last) > idleTimeout {
		return false
	}
	lastSeen[key] = now
	return true
}

// sessionKeyOf scopes idleness per browser session (JWT session_id claim) so a
// fresh login is not blocked by an old idle session for the same user.
func sessionKeyOf(userID, token string) string {
	parts := strings.Split(token, ".")
	if len(parts) == 3 {
		if raw, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			var claims struct {
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal(raw, &claims) == nil && claims.SessionID != "" {
				return userID + ":" + claims.SessionID
			}
		}
	}
	return userID
}

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
			if !active(sessionKeyOf(id, token)) {
				httpx.WriteError(w, 401, "Sesi tidak aktif selama 2 jam. Silakan masuk kembali.")
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, Identity{ID: id, Email: email})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
