package middleware

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestActiveIdleTimeout(t *testing.T) {
	key := "u1:session-a"
	if !active(key) || !active(key) {
		t.Fatal("fresh session should stay active")
	}
	idleMu.Lock()
	lastSeen[key] = time.Now().Add(-2*time.Hour - time.Minute)
	idleMu.Unlock()
	if active(key) {
		t.Fatal("session idle >2h must expire")
	}
	if !active("u1:session-b") {
		t.Fatal("new login session must not inherit old idle state")
	}
}

func TestSessionKeyOf(t *testing.T) {
	if got := sessionKeyOf("u1", "not-a-jwt"); got != "u1" {
		t.Fatalf("invalid token should fall back to user id, got %s", got)
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"session_id":"abc"}`))
	if got := sessionKeyOf("u1", "header."+payload+".sig"); got != "u1:abc" {
		t.Fatalf("want u1:abc, got %s", got)
	}
}
