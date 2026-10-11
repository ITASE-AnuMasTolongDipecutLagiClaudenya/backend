package handler

import (
	"net/http"
	"strings"

	"nutrily/backend/internal/httpx"
)

func (h *Handler) VerifyTurnstile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "Method tidak didukung")
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if !httpx.DecodeBody(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Token) == "" || len(body.Token) > 2048 {
		httpx.WriteError(w, 400, "Token Turnstile diperlukan")
		return
	}
	if h.TS == nil || h.TS.Secret == "" {
		httpx.WriteError(w, 503, "Verifikasi keamanan belum dikonfigurasi")
		return
	}
	ok, err := h.TS.Verify(r.Context(), body.Token, "")
	if err != nil {
		httpx.WriteError(w, 502, "Verifikasi Turnstile gagal")
		return
	}
	if !ok {
		httpx.WriteError(w, 403, "Verifikasi keamanan gagal")
		return
	}
	httpx.WriteJSON(w, 200, map[string]bool{"ok": true})
}
