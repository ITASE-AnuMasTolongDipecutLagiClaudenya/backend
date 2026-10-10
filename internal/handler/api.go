package handler

import (
	"net/http"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/middleware"
)

func (h *Handler) ServeAPI(w http.ResponseWriter, r *http.Request) {
	u := middleware.FromContext(r.Context())
	switch {
	case r.URL.Path == "/api/profile" && r.Method == "GET":
		h.GetProfile(w, r, u)
	case r.URL.Path == "/api/profile" && r.Method == "PUT":
		h.PutProfile(w, r, u)
	case r.URL.Path == "/api/dashboard" && r.Method == "GET":
		h.Dashboard(w, r, u)
	case r.URL.Path == "/api/meals" && r.Method == "GET":
		h.ListMeals(w, r, u)
	case r.URL.Path == "/api/meals" && r.Method == "POST":
		h.CreateMeal(w, r, u)
	case r.URL.Path == "/api/scan" && r.Method == "POST":
		h.Scan(w, r, u)
	case r.URL.Path == "/api/chat" && r.Method == "GET":
		h.GetChat(w, r, u)
	case r.URL.Path == "/api/conversations" && r.Method == "GET":
		h.ListConversations(w, r, u)
	case r.URL.Path == "/api/chat" && r.Method == "POST":
		h.PostChat(w, r, u)
	default:
		httpx.WriteError(w, 404, "Endpoint tidak ditemukan")
	}
}
