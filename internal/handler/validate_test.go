package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidUUID(t *testing.T) {
	if !ValidUUID("550e8400-e29b-41d4-a716-446655440000") || ValidUUID("not-a-uuid") {
		t.Fatal("UUID validation failed")
	}
}

func TestDeleteConversationRejectsInvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/conversations?conversation_id=invalid", nil)
	response := httptest.NewRecorder()
	(&Handler{}).ServeAPI(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("DELETE invalid conversation status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
