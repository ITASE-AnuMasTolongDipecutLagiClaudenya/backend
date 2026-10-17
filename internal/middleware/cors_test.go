package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsPatchPreflight(t *testing.T) {
	handler := CORS(map[string]bool{"http://localhost:8443": true})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	request := httptest.NewRequest(http.MethodOptions, "/api/meals", nil)
	request.Header.Set("Origin", "http://localhost:8443")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if methods := response.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(methods, http.MethodPatch) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want PATCH included", methods)
	}
}
