package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nutrily/backend/internal/middleware"
)

func TestCreateMealRejectsInvalidSessionName(t *testing.T) {
	for _, sessionName := range []string{"", "   ", strings.Repeat("é", 61)} {
		t.Run("invalid", func(t *testing.T) {
			body, err := json.Marshal(map[string]string{
				"name":              "Makanan",
				"scan_session_name": sessionName,
			})
			if err != nil {
				t.Fatal(err)
			}

			request := httptest.NewRequest(http.MethodPost, "/api/meals", strings.NewReader(string(body)))
			response := httptest.NewRecorder()
			(&Handler{}).CreateMeal(response, request, middleware.Identity{ID: "user-1"})
			if response.Code != http.StatusBadRequest {
				t.Fatalf("CreateMeal status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}
