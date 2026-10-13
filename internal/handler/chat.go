package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
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

func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if !ValidUUID(conversationID) {
		httpx.WriteError(w, 400, "ID percakapan tidak valid")
		return
	}
	_, err := h.SB.Query(r.Context(), r.Header.Get("Authorization"), "DELETE", "chat_messages", supabase.UserFilter(u.ID)+"&conversation_id=eq."+url.QueryEscape(conversationID))
	if err != nil {
		log.Printf("conversation delete: %v", err)
		if strings.Contains(err.Error(), "42501") || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
			httpx.WriteError(w, 502, "Database menolak hapus riwayat. Pastikan migration 003_chat_delete.sql sudah dijalankan pada project Supabase yang dipakai aplikasi.")
			return
		}
		httpx.WriteError(w, 502, "Gagal menghapus percakapan: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PostChat(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	if !h.HasAI() {
		httpx.WriteError(w, 503, "Chat memerlukan GEMINI_API_KEY atau OPENROUTER_API_KEY di backend")
		return
	}
	var body struct {
		Message        string `json:"message"`
		ConversationID string `json:"conversation_id"`
	}
	var imageData []byte
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
		if err := r.ParseMultipartForm(9 << 20); err != nil {
			httpx.WriteError(w, 400, "Gambar tidak valid atau melebihi 8 MB")
			return
		}
		body.Message = r.FormValue("message")
		body.ConversationID = r.FormValue("conversation_id")
		if file, _, err := r.FormFile("image"); err == nil {
			defer file.Close()
			imageData, err = io.ReadAll(io.LimitReader(file, 8<<20+1))
			if err != nil || len(imageData) > 8<<20 {
				httpx.WriteError(w, 400, "Gambar melebihi 8 MB")
				return
			}
			if _, _, err := image.DecodeConfig(bytes.NewReader(imageData)); err != nil {
				httpx.WriteError(w, 400, "Konten file bukan gambar yang valid")
				return
			}
			contentType = http.DetectContentType(imageData)
			if contentType != "image/jpeg" && contentType != "image/png" {
				httpx.WriteError(w, 400, "Gunakan gambar JPG atau PNG")
				return
			}
		} else if r.MultipartForm.File["image"] != nil {
			httpx.WriteError(w, 400, "File gambar tidak valid")
			return
		}
	} else if !httpx.DecodeBody(w, r, &body) {
		return
	}
	body.Message = strings.TrimSpace(body.Message)
	if (body.Message == "" && len(imageData) == 0) || len(body.Message) > 4000 {
		httpx.WriteError(w, 400, "Pesan harus berisi teks maksimal 4000 karakter atau gambar")
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
	userParts := []any{map[string]string{"text": body.Message}}
	if len(imageData) > 0 {
		userParts = append(userParts, map[string]any{"inlineData": map[string]string{"mimeType": contentType, "data": base64.StdEncoding.EncodeToString(imageData)}})
	}
	messages = append(messages, map[string]any{"role": "user", "parts": userParts})
	response, err := h.GenerateAI(r.Context(), systemInstruction, messages, false)
	if err != nil {
		httpx.WriteError(w, 502, "Asisten AI gagal merespons")
		return
	}
	answer := fmt.Sprint(response["answer"])
	storedMessage := body.Message
	if len(imageData) > 0 {
		storedMessage = strings.TrimSpace(storedMessage + "\n[Gambar terlampir]")
	}
	_, saveErr := h.SB.Query(r.Context(), auth, "POST", "chat_messages", []map[string]any{
		{"user_id": u.ID, "conversation_id": body.ConversationID, "role": "user", "content": storedMessage},
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
