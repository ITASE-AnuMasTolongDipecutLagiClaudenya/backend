package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/middleware"
	"nutrily/backend/internal/nutrition"
	"nutrily/backend/internal/supabase"
)

func (h *Handler) GetChat(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if conversationID == "" {
		rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "chat_messages", supabase.UserFilter(u.ID)+"&select=conversation_id,created_at&order=created_at.desc&limit=1")
		if err != nil {
			httpx.WriteError(w, 502, "Gagal membaca riwayat chat")
			return
		}
		if len(rows) == 0 {
			httpx.WriteJSON(w, 200, map[string]any{"conversation_id": "", "messages": []any{}})
			return
		}
		conversationID = fmt.Sprint(rows[0]["conversation_id"])
	}
	if !ValidUUID(conversationID) {
		httpx.WriteError(w, 400, "ID percakapan tidak valid")
		return
	}
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "chat_messages", supabase.UserFilter(u.ID)+"&conversation_id=eq."+url.QueryEscape(conversationID)+"&select=id,role,content,created_at&order=created_at.asc&limit=100")
	if err != nil {
		log.Printf("chat history query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca riwayat chat")
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"conversation_id": conversationID, "messages": rows})
}

func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	rows, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "GET", "chat_messages", supabase.UserFilter(u.ID)+"&select=conversation_id,content,created_at&order=created_at.desc&limit=500")
	if err != nil {
		log.Printf("conversation list query: %v", err)
		httpx.WriteError(w, 502, "Gagal membaca daftar percakapan")
		return
	}
	seen := map[string]bool{}
	conversations := []map[string]any{}
	for _, row := range rows {
		id := fmt.Sprint(row["conversation_id"])
		if seen[id] {
			continue
		}
		seen[id] = true
		conversations = append(conversations, map[string]any{"id": id, "preview": row["content"], "updated_at": row["created_at"]})
	}
	httpx.WriteJSON(w, 200, conversations)
}

func (h *Handler) PostChat(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	if h.AI.APIKey == "" {
		httpx.WriteError(w, 503, "Chat AI memerlukan GEMINI_API_KEY di backend")
		return
	}
	var body struct {
		Message        string `json:"message"`
		ConversationID string `json:"conversation_id"`
	}
	if !httpx.DecodeBody(w, r, &body) {
		return
	}
	body.Message = strings.TrimSpace(body.Message)
	if body.Message == "" || len(body.Message) > 4000 {
		httpx.WriteError(w, 400, "Pesan harus berisi 1–4000 karakter")
		return
	}
	if !ValidUUID(body.ConversationID) {
		httpx.WriteError(w, 400, "ID percakapan tidak valid")
		return
	}
	auth := r.Header.Get("Authorization")
	profile, err := h.SB.Query(r.Context(), auth, "GET", "profiles", supabase.UserFilter(u.ID)+"&select=age,gender,goal,weight_kg,height_cm")
	if err != nil {
		httpx.WriteError(w, 502, "Gagal membaca konteks profil")
		return
	}
	profileContext, _ := json.Marshal(nutrition.First(profile))
	systemInstruction := "Kamu asisten gizi Nutrily untuk sesi ringkasan pemindaian ini. Hanya bahas gizi, makanan, dan kesehatan terkait sesi makan pengguna dan riwayat percakapan ini. Jika pertanyaan di luar gizi/kesehatan, tolak singkat dan arahkan kembali ke topik sesi. Jawab dalam bahasa yang digunakan pengguna. Beri saran umum yang aman, jangan mendiagnosis. Profil pengguna: " + string(profileContext)
	history, err := h.SB.Query(r.Context(), auth, "GET", "chat_messages", supabase.UserFilter(u.ID)+"&conversation_id=eq."+url.QueryEscape(body.ConversationID)+"&select=role,content&order=created_at.desc&limit=20")
	if err != nil {
		httpx.WriteError(w, 502, "Gagal membaca konteks percakapan")
		return
	}
	messages := []any{}
	for i := len(history) - 1; i >= 0; i-- {
		role := "user"
		if history[i]["role"] == "assistant" {
			role = "model"
		}
		messages = append(messages, map[string]any{"role": role, "parts": []any{map[string]any{"text": history[i]["content"]}}})
	}
	messages = append(messages, map[string]any{"role": "user", "parts": []any{map[string]string{"text": body.Message}}})
	response, err := h.AI.Generate(r.Context(), systemInstruction, messages, false)
	if err != nil {
		httpx.WriteError(w, 502, "Asisten AI gagal merespons")
		return
	}
	answer := fmt.Sprint(response["answer"])
	_, saveErr := h.SB.Query(r.Context(), auth, "POST", "chat_messages", []map[string]any{
		{"user_id": u.ID, "conversation_id": body.ConversationID, "role": "user", "content": body.Message},
		{"user_id": u.ID, "conversation_id": body.ConversationID, "role": "assistant", "content": answer},
	})
	if saveErr != nil {
		log.Printf("chat save: %v", saveErr)
		httpx.WriteError(w, 502, "Respons dibuat, tetapi gagal menyimpan riwayat chat")
		return
	}
	if !strings.HasPrefix(body.Message, "Saya baru memindai makanan berikut:") {
		h.recordFeatureUsage(r, "chatbot")
	}
	httpx.WriteJSON(w, 200, map[string]string{"answer": answer})
}
