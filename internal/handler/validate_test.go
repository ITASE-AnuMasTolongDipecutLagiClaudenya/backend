package handler

import "testing"

func TestValidUUID(t *testing.T) {
	if !ValidUUID("550e8400-e29b-41d4-a716-446655440000") || ValidUUID("not-a-uuid") {
		t.Fatal("UUID validation failed")
	}
}
