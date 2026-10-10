package server

import (
	"net/http"

	"nutrily/backend/internal/config"
	"nutrily/backend/internal/handler"
	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/middleware"
	"nutrily/backend/internal/supabase"
)

func New(cfg config.Config, sb *supabase.Client, h *handler.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.Handle("/api/", middleware.CORS(cfg.Origins)(middleware.Authenticate(sb)(http.HandlerFunc(h.ServeAPI))))
	return mux
}
