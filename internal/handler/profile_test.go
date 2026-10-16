package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nutrily/backend/internal/supabase"
)

type profileTestRoundTripFunc func(*http.Request) (*http.Response, error)

func (f profileTestRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestPutProfileSavesGoalWithoutAvatarUpload(t *testing.T) {
	client := &supabase.Client{
		BaseURL: "https://example.supabase.co",
		HTTP: &http.Client{Transport: profileTestRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost || r.URL.Path != "/rest/v1/profiles" {
				t.Errorf("unexpected Supabase request: %s %s", r.Method, r.URL.String())
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), `"goal":"Menjaga berat badan"`) {
				t.Errorf("profile request missing goal: %s", body)
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`[{}]`)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		})},
	}
	h := &Handler{SB: client}
	body := `{"full_name":"Test User","age":30,"gender":"Pria","weight_kg":70,"height_cm":175,"activity":"Sedang","goal":"Menjaga berat badan","is_gym":false}`
	r := httptest.NewRequest(http.MethodPut, "/api/profile", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeAPI(w, r)

	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{}` {
		t.Fatalf("PUT profile response = %d %s", w.Code, w.Body.String())
	}
}
