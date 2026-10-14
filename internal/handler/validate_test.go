package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nutrily/backend/internal/middleware"
	"nutrily/backend/internal/supabase"
)

type deleteRoundTripFunc func(*http.Request) (*http.Response, error)

func (f deleteRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

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

func TestGetChatRequiresConversationID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/chat", nil)
	response := httptest.NewRecorder()
	(&Handler{}).ServeAPI(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("GET chat without conversation ID status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestGetChatFiltersUserAndConversation(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: deleteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet || r.URL.Query().Get("user_id") != "eq.user-1" || r.URL.Query().Get("conversation_id") != "eq.550e8400-e29b-41d4-a716-446655440000" {
				t.Errorf("unexpected history request: %s %s", r.Method, r.URL.String())
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header), Request: r}, nil
		})},
	}
	h := &Handler{SB: client}
	r := httptest.NewRequest(http.MethodGet, "/api/chat?conversation_id=550e8400-e29b-41d4-a716-446655440000", nil)
	w := httptest.NewRecorder()
	h.GetChat(w, r, middleware.Identity{ID: "user-1"})
	if w.Code != http.StatusOK {
		t.Fatalf("GET chat status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestDeleteConversationFiltersUserAndSession(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: deleteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodDelete || r.URL.Query().Get("user_id") != "eq.user-1" || r.URL.Query().Get("conversation_id") != "eq.550e8400-e29b-41d4-a716-446655440000" {
				t.Errorf("unexpected delete request: %s %s", r.Method, r.URL.String())
			}
			if got := r.Header.Get("Prefer"); got != "return=representation" {
				t.Errorf("Prefer = %q, want return=representation", got)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`[{"id":1}]`)), Header: make(http.Header), Request: r}, nil
		})},
	}
	h := &Handler{SB: client}
	r := httptest.NewRequest(http.MethodDelete, "/api/conversations?conversation_id=550e8400-e29b-41d4-a716-446655440000", nil)
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	h.DeleteConversation(w, r, middleware.Identity{ID: "user-1"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE conversation status = %d, want %d: %s", w.Code, http.StatusNoContent, w.Body.String())
	}
}

func TestDeleteConversationReportsNoRowsDeleted(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: deleteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("[]")), Header: make(http.Header), Request: r}, nil
		})},
	}
	h := &Handler{SB: client}
	r := httptest.NewRequest(http.MethodDelete, "/api/conversations?conversation_id=550e8400-e29b-41d4-a716-446655440000", nil)
	w := httptest.NewRecorder()
	h.DeleteConversation(w, r, middleware.Identity{ID: "user-1"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("DELETE empty conversation status = %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestDeleteConversationReportsMissingSupabasePermission(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: deleteRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader(`{"code":"42501","message":"permission denied"}`)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		})},
	}
	h := &Handler{SB: client}
	r := httptest.NewRequest(http.MethodDelete, "/api/conversations?conversation_id=550e8400-e29b-41d4-a716-446655440000", nil)
	w := httptest.NewRecorder()
	h.DeleteConversation(w, r, middleware.Identity{ID: "user-1"})
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "005_chat_delete_policy.sql") {
		t.Fatalf("DELETE permission error = %d %s", w.Code, w.Body.String())
	}
}
