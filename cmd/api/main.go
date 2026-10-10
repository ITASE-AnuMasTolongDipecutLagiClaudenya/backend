package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type server struct {
	supabaseURL, anonKey, geminiKey, model string
	client                                 *http.Client
}

type identity struct{ id, email string }
type identityKey struct{}

func main() {
	loadEnv()
	s := &server{
		supabaseURL: strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		anonKey:     os.Getenv("SUPABASE_ANON_KEY"),
		geminiKey:   os.Getenv("GEMINI_API_KEY"),
		model:       env("GEMINI_MODEL", "gemini-3.8-flash"),
		client:      &http.Client{Timeout: 45 * time.Second},
	}
	if s.supabaseURL == "" || s.anonKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_ANON_KEY are required")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.Handle("/api/", s.cors(s.authenticate(http.HandlerFunc(s.handleAPI))))
	addr := ":" + env("PORT", "8080")
	log.Printf("Nutrily API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || os.Getenv(key) != "" {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, "\"'")
		}
		_ = os.Setenv(key, value)
	}
}

func (s *server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{"http://localhost:5173": true, "http://localhost:8443": true}
	for _, origin := range strings.Split(os.Getenv("FRONTEND_URL"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if token == "" {
			writeError(w, 401, "Bearer token diperlukan")
			return
		}
		req, _ := http.NewRequestWithContext(r.Context(), "GET", s.supabaseURL+"/auth/v1/user", nil)
		req.Header.Set("apikey", s.anonKey)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := s.client.Do(req)
		if err != nil {
			writeError(w, 502, "Validasi sesi Supabase gagal")
			return
		}
		defer resp.Body.Close()
		var user struct{ ID, Email string }
		if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user) != nil || user.ID == "" {
			writeError(w, 401, "Sesi tidak valid atau kedaluwarsa")
			return
		}
		ctx := context.WithValue(r.Context(), identityKey{}, identity{id: user.ID, email: user.Email})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) handleAPI(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(identityKey{}).(identity)
	switch {
	case r.URL.Path == "/api/profile" && r.Method == "GET":
		s.getProfile(w, r, user)
	case r.URL.Path == "/api/profile" && r.Method == "PUT":
		s.putProfile(w, r, user)
	case r.URL.Path == "/api/dashboard" && r.Method == "GET":
		s.dashboard(w, r, user)
	case r.URL.Path == "/api/meals" && r.Method == "GET":
		s.listMeals(w, r, user)
	case r.URL.Path == "/api/meals" && r.Method == "POST":
		s.createMeal(w, r, user)
	case r.URL.Path == "/api/scan" && r.Method == "POST":
		s.scan(w, r, user)
	case r.URL.Path == "/api/chat" && r.Method == "GET":
		s.getChat(w, r, user)
	case r.URL.Path == "/api/conversations" && r.Method == "GET":
		s.listConversations(w, r, user)
	case r.URL.Path == "/api/chat" && r.Method == "POST":
		s.postChat(w, r, user)
	default:
		writeError(w, 404, "Endpoint tidak ditemukan")
	}
}

func (s *server) getProfile(w http.ResponseWriter, r *http.Request, u identity) {
	rows, err := s.db(r, "GET", "profiles", userFilter(u.id)+"&select=*")
	if err != nil {
		log.Printf("profile query: %v", err)
		writeError(w, 502, "Gagal membaca profil")
		return
	}
	if len(rows) == 0 {
		writeJSON(w, 200, map[string]any{})
		return
	}
	writeJSON(w, 200, rows[0])
}

func (s *server) putProfile(w http.ResponseWriter, r *http.Request, u identity) {
	var body map[string]any
	if !decodeBody(w, r, &body) {
		return
	}
	if body == nil {
		writeError(w, 400, "Body profil wajib berupa objek JSON")
		return
	}
	for field, limits := range map[string][2]float64{"age": {13, 120}, "weight_kg": {1, 500}, "height_cm": {30, 275}} {
		value, ok := body[field].(float64)
		if !ok || value < limits[0] || value > limits[1] {
			writeError(w, 400, "Nilai "+field+" tidak valid")
			return
		}
	}
	if gender, ok := body["gender"].(string); ok && gender != "Pria" && gender != "Wanita" {
		writeError(w, 400, "Jenis kelamin tidak valid")
		return
	}
	if body["gender"] != "Pria" && body["gender"] != "Wanita" {
		writeError(w, 400, "Jenis kelamin wajib diisi")
		return
	}
	if activity, ok := body["activity"].(string); !ok || !oneOf(activity, "Minim gerak", "Ringan", "Sedang", "Sangat aktif", "Aktivitas fisik berat") {
		writeError(w, 400, "Aktivitas tidak valid")
		return
	}
	if goal, ok := body["goal"].(string); !ok || !oneOf(goal, "Menurunkan berat badan", "Menambah berat badan") {
		writeError(w, 400, "Target tidak valid")
		return
	}
	if name, ok := body["full_name"].(string); !ok || len(strings.TrimSpace(name)) > 120 {
		writeError(w, 400, "Nama profil tidak valid")
		return
	}
	if status, ok := body["pregnancy_status"].(string); ok && !oneOf(status, "Tidak ada", "Trimester 1", "Trimester 2", "Trimester 3", "Menyusui 6 bulan pertama", "Menyusui 6 bulan kedua") {
		writeError(w, 400, "Status kehamilan tidak valid")
		return
	}
	allowed := map[string]bool{"user_id": true, "age": true, "gender": true, "pregnancy_status": true, "weight_kg": true, "height_cm": true, "activity": true, "goal": true, "full_name": true}
	for key := range body {
		if !allowed[key] {
			writeError(w, 400, "Field profil tidak dikenal: "+key)
			return
		}
	}
	body["user_id"] = u.id
	if name, ok := body["full_name"].(string); ok {
		body["full_name"] = strings.TrimSpace(name)
	} else {
		body["full_name"] = ""
	}
	rows, err := s.db(r, "POST", "profiles?on_conflict=user_id", body)
	if err != nil || len(rows) == 0 {
		if err != nil {
			log.Printf("profile save: %v", err)
		}
		writeError(w, 502, "Gagal menyimpan profil; periksa skema Supabase")
		return
	}
	writeJSON(w, 200, rows[0])
}

func (s *server) dashboard(w http.ResponseWriter, r *http.Request, u identity) {
	profile, err := s.db(r, "GET", "profiles", userFilter(u.id)+"&select=*")
	if err != nil {
		log.Printf("dashboard profile query: %v", err)
		writeError(w, 502, "Gagal membaca profil")
		return
	}
	today := time.Now().In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02")
	meals, err := s.db(r, "GET", "meals", userFilter(u.id)+"&created_at=gte."+url.QueryEscape(today+"T00:00:00+07:00")+"&order=created_at.desc&limit=100")
	if err != nil {
		log.Printf("dashboard meals query: %v", err)
		writeError(w, 502, "Gagal membaca catatan makan")
		return
	}
	totals := map[string]float64{"calories": 0, "protein_g": 0, "carbs_g": 0, "fat_g": 0}
	for _, meal := range meals {
		for _, key := range []string{"calories", "protein_g", "carbs_g", "fat_g"} {
			if n, ok := meal[key].(float64); ok {
				totals[key] += n
			}
		}
	}
	writeJSON(w, 200, map[string]any{"profile": first(profile), "meals": meals, "totals": totals, "targets": nutritionTargets(first(profile))})
}

func (s *server) listMeals(w http.ResponseWriter, r *http.Request, u identity) {
	rows, err := s.db(r, "GET", "meals", userFilter(u.id)+"&order=created_at.desc&limit=100")
	if err != nil {
		log.Printf("meal list query: %v", err)
		writeError(w, 502, "Gagal membaca catatan makan")
		return
	}
	writeJSON(w, 200, rows)
}

func (s *server) createMeal(w http.ResponseWriter, r *http.Request, u identity) {
	var body map[string]any
	if !decodeBody(w, r, &body) {
		return
	}
	if body == nil {
		writeError(w, 400, "Body makanan wajib berupa objek JSON")
		return
	}
	name, ok := body["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		writeError(w, 400, "Nama makanan wajib diisi")
		return
	}
	if len(name) > 160 {
		writeError(w, 400, "Nama makanan terlalu panjang")
		return
	}
	allowed := map[string]bool{"user_id": true, "name": true, "serving_size": true, "calories": true, "protein_g": true, "carbs_g": true, "fat_g": true, "sugar_g": true, "sodium_mg": true}
	for key := range body {
		if !allowed[key] {
			writeError(w, 400, "Field makanan tidak dikenal: "+key)
			return
		}
	}
	for _, field := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
		if raw, exists := body[field]; exists {
			value, ok := raw.(float64)
			if !ok || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				writeError(w, 400, "Nilai "+field+" harus berupa angka positif")
				return
			}
		}
	}
	body["user_id"] = u.id
	rows, err := s.db(r, "POST", "meals", body)
	if err != nil || len(rows) == 0 {
		if err != nil {
			log.Printf("meal save: %v", err)
		}
		writeError(w, 502, "Gagal menyimpan makanan; periksa skema Supabase")
		return
	}
	writeJSON(w, 201, rows[0])
}

func (s *server) scan(w http.ResponseWriter, r *http.Request, u identity) {
	if s.geminiKey == "" {
		writeError(w, 503, "Pemindaian memerlukan GEMINI_API_KEY di backend")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if err := r.ParseMultipartForm(9 << 20); err != nil {
		writeError(w, 400, "Gambar tidak valid atau melebihi 8 MB")
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, 400, "File gambar wajib disertakan")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil || len(data) > 8<<20 {
		writeError(w, 400, "Gambar melebihi 8 MB")
		return
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		writeError(w, 400, "Konten file bukan gambar yang valid")
		return
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" {
		writeError(w, 400, "Gunakan gambar JPG atau PNG")
		return
	}
	mode := strings.TrimSpace(r.FormValue("mode"))
	if mode == "" {
		mode = "label"
	}
	if mode != "label" && mode != "receipt" {
		writeError(w, 400, "Mode pindai tidak valid")
		return
	}
	prompt := "Baca label gizi pada gambar. Balas HANYA JSON valid tanpa markdown: {\"name\":..., \"serving_size\":..., \"calories\":..., \"protein_g\":..., \"carbs_g\":..., \"fat_g\":..., \"sugar_g\":..., \"sodium_mg\":..., \"note\":...}. Gunakan angka; null untuk nilai tidak terbaca. Jangan mengarang."
	if mode == "receipt" {
		prompt = "Baca seluruh struk, termasuk bagian atas dan bawah. Periksa setiap baris barang satu per satu: OCR nama barang seperti tertulis, termasuk nama merek, singkatan, dan ejaan yang kurang jelas. Masukkan setiap baris yang mungkin merupakan makanan, minuman, atau buah untuk dikonsumsi, termasuk barang kemasan, snack, roti, mi, susu, kopi, teh, jus, air minum, dan buah. Jangan buang barang hanya karena nama/merek tidak dikenal atau teks struk disingkat; jika ragu apakah barang dapat dikonsumsi, tetap masukkan dan beri note \"Nama perlu diverifikasi\". Jangan gabungkan baris barang yang berbeda. Abaikan hanya baris yang jelas bukan barang konsumsi serta pajak, diskon, subtotal, total, kembalian, metode pembayaran, biaya layanan, dan kantong belanja. Jika jumlah N terbaca (misal 2x), kalikan semua angka gizi dengan N dan tulis serving_size seperti \"2x ...\"; jangan buat field jumlah terpisah. Estimasi gizi per sajian standar Indonesia; jangan menghilangkan barang karena nilai gizinya tidak diketahui, gunakan null untuk nilai yang tidak dapat diestimasi. Balas HANYA JSON valid tanpa markdown: {\"items\": [{\"name\":..., \"category\": \"makanan|minuman|buah\", \"serving_size\":..., \"calories\":..., \"protein_g\":..., \"carbs_g\":..., \"fat_g\":..., \"sugar_g\":..., \"sodium_mg\":..., \"note\":...}]}. Satu objek untuk setiap baris barang konsumsi. Jika tidak ada, balas {\"items\": []}. Maksimal 50 barang."
	}
	contents := []any{map[string]any{"role": "user", "parts": []any{
		map[string]string{"text": prompt},
		map[string]any{"inlineData": map[string]string{"mimeType": contentType, "data": base64.StdEncoding.EncodeToString(data)}},
	}}}
	result, err := s.generate(r.Context(), "", contents, true)
	if err != nil {
		writeError(w, 502, "Pemindaian gagal: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"mode":    mode,
		"items":   normalizeScanItems(result),
		"user_id": u.id,
	})
}

func (s *server) getChat(w http.ResponseWriter, r *http.Request, u identity) {
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if conversationID == "" {
		var rows []map[string]any
		rows, err := s.db(r, "GET", "chat_messages", userFilter(u.id)+"&select=conversation_id,created_at&order=created_at.desc&limit=1")
		if err != nil {
			writeError(w, 502, "Gagal membaca riwayat chat")
			return
		}
		if len(rows) == 0 {
			writeJSON(w, 200, map[string]any{"conversation_id": "", "messages": []any{}})
			return
		}
		conversationID = fmt.Sprint(rows[0]["conversation_id"])
	}
	if !validUUID(conversationID) {
		writeError(w, 400, "ID percakapan tidak valid")
		return
	}
	rows, err := s.db(r, "GET", "chat_messages", userFilter(u.id)+"&conversation_id=eq."+url.QueryEscape(conversationID)+"&select=id,role,content,created_at&order=created_at.asc&limit=100")
	if err != nil {
		log.Printf("chat history query: %v", err)
		writeError(w, 502, "Gagal membaca riwayat chat")
		return
	}
	writeJSON(w, 200, map[string]any{"conversation_id": conversationID, "messages": rows})
}

func (s *server) listConversations(w http.ResponseWriter, r *http.Request, u identity) {
	rows, err := s.db(r, "GET", "chat_messages", userFilter(u.id)+"&select=conversation_id,content,created_at&order=created_at.desc&limit=500")
	if err != nil {
		log.Printf("conversation list query: %v", err)
		writeError(w, 502, "Gagal membaca daftar percakapan")
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
	writeJSON(w, 200, conversations)
}

func (s *server) postChat(w http.ResponseWriter, r *http.Request, u identity) {
	if s.geminiKey == "" {
		writeError(w, 503, "Chat AI memerlukan GEMINI_API_KEY di backend")
		return
	}
	var body struct {
		Message        string `json:"message"`
		ConversationID string `json:"conversation_id"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	body.Message = strings.TrimSpace(body.Message)
	if body.Message == "" || len(body.Message) > 4000 {
		writeError(w, 400, "Pesan harus berisi 1–4000 karakter")
		return
	}
	if !validUUID(body.ConversationID) {
		writeError(w, 400, "ID percakapan tidak valid")
		return
	}
	profile, err := s.db(r, "GET", "profiles", userFilter(u.id)+"&select=age,gender,goal,weight_kg,height_cm")
	if err != nil {
		writeError(w, 502, "Gagal membaca konteks profil")
		return
	}
	profileContext, _ := json.Marshal(first(profile))
	systemInstruction := "Kamu asisten gizi Nutrily untuk sesi ringkasan pemindaian ini. Hanya bahas gizi, makanan, dan kesehatan terkait sesi makan pengguna dan riwayat percakapan ini. Jika pertanyaan di luar gizi/kesehatan, tolak singkat dan arahkan kembali ke topik sesi. Jawab dalam bahasa yang digunakan pengguna. Beri saran umum yang aman, jangan mendiagnosis. Profil pengguna: " + string(profileContext)
	history, err := s.db(r, "GET", "chat_messages", userFilter(u.id)+"&conversation_id=eq."+url.QueryEscape(body.ConversationID)+"&select=role,content&order=created_at.desc&limit=20")
	if err != nil {
		writeError(w, 502, "Gagal membaca konteks percakapan")
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
	response, err := s.generate(r.Context(), systemInstruction, messages, false)
	if err != nil {
		writeError(w, 502, "Asisten AI gagal merespons")
		return
	}
	answer := fmt.Sprint(response["answer"])
	_, saveErr := s.db(r, "POST", "chat_messages", []map[string]any{
		{"user_id": u.id, "conversation_id": body.ConversationID, "role": "user", "content": body.Message},
		{"user_id": u.id, "conversation_id": body.ConversationID, "role": "assistant", "content": answer},
	})
	if saveErr != nil {
		log.Printf("chat save: %v", saveErr)
		writeError(w, 502, "Respons dibuat, tetapi gagal menyimpan riwayat chat")
		return
	}
	writeJSON(w, 200, map[string]string{"answer": answer})
}

func (s *server) generate(ctx context.Context, systemInstruction string, contents []any, jsonMode bool) (map[string]any, error) {
	payload := map[string]any{"contents": contents, "generationConfig": map[string]any{"temperature": 0.2}}
	if systemInstruction != "" {
		payload["systemInstruction"] = map[string]any{"parts": []any{map[string]string{"text": systemInstruction}}}
	}
	if jsonMode {
		payload["generationConfig"].(map[string]any)["responseMimeType"] = "application/json"
	}
	body, _ := json.Marshal(payload)
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(s.model) + ":generateContent"
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", s.geminiKey)
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusServiceUnavailable && attempt < 2 {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			timer := time.NewTimer(time.Duration(1<<attempt) * 500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
				continue
			}
		}
		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			return nil, fmt.Errorf("provider HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		text, err := geminiText(data)
		if err != nil {
			return nil, err
		}
		if jsonMode {
			cleaned := stripJSONFence(text)
			var scan map[string]any
			if err := json.Unmarshal([]byte(cleaned), &scan); err != nil {
				var list []any
				if err := json.Unmarshal([]byte(cleaned), &list); err != nil {
					return nil, errors.New("hasil OCR bukan JSON valid")
				}
				return map[string]any{"items": list}, nil
			}
			return scan, nil
		}
		return map[string]any{"answer": text}, nil
	}
	return nil, errors.New("layanan Gemini tidak tersedia")
}

func geminiText(data []byte) (string, error) {
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Candidates) == 0 {
		return "", errors.New("respons Gemini tidak valid")
	}
	var content strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		content.WriteString(part.Text)
	}
	text := strings.TrimSpace(content.String())
	if text == "" {
		return "", errors.New("Gemini tidak menghasilkan teks; coba gambar atau prompt lain")
	}
	return text, nil
}

func (s *server) db(r *http.Request, method, path string, body ...any) ([]map[string]any, error) {
	var reader io.Reader
	if len(body) > 0 {
		encoded, err := json.Marshal(body[0])
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, s.supabaseURL+"/rest/v1/"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", s.anonKey)
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
		if method == "POST" && strings.Contains(path, "on_conflict=") {
			req.Header.Set("Prefer", "resolution=merge-duplicates,return=representation")
		} else {
			req.Header.Set("Prefer", "return=representation")
		}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Supabase REST HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if len(data) == 0 {
		return nil, nil
	}
	var rows []map[string]any
	err = json.Unmarshal(data, &rows)
	return rows, err
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		writeError(w, 400, "Body JSON tidak valid")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, 400, "Body harus berisi satu objek JSON")
		return false
	}
	return true
}

func userFilter(id string) string { return "user_id=eq." + url.QueryEscape(id) }
func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func nutritionTargets(profile map[string]any) map[string]float64 {
	weight := toFloat(profile["weight_kg"])
	height := toFloat(profile["height_cm"])
	age := toFloat(profile["age"])
	if weight <= 0 || height <= 0 || age <= 0 {
		return map[string]float64{"calories": 2100, "protein_g": 100, "carbs_g": 260, "fat_g": 70}
	}
	sexOffset := -161.0
	if profile["gender"] == "Pria" {
		sexOffset = 5
	}
	bmr := 10*weight + 6.25*height - 5*age + sexOffset
	activity := 1.375
	switch profile["activity"] {
	case "Minim gerak":
		activity = 1.2
	case "Sedang":
		activity = 1.55
	case "Sangat aktif":
		activity = 1.725
	case "Aktivitas fisik berat":
		activity = 1.9
	}
	calories := bmr * activity
	switch profile["goal"] {
	case "Menurunkan berat badan":
		calories -= 300
	case "Menambah berat badan":
		calories += 250
	}
	calories = math.Max(1200, math.Min(5000, calories))
	protein := weight * 1.6
	fat := calories * .25 / 9
	return map[string]float64{"calories": math.Round(calories), "protein_g": math.Round(protein), "fat_g": math.Round(fat), "carbs_g": math.Round((calories - protein*4 - fat*9) / 4)}
}

func first(rows []map[string]any) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	return rows[0]
}

func toFloat(value any) float64 {
	switch n := value.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

var nonFoodKeywords = []string{
	"pajak", "ppn", "diskon", "discount", "total", "subtotal", "sub total", "kembalian", "kembali",
	"tunai", "cash", "debit", "kredit", "credit", "qr", "qris", "layanan", "service", "kantong",
	"sabun", "sampo", "shampoo", "deterjen", "detergen", "pasta gigi", "sikat gigi", "tisu", "tissue",
	"popok", "diapers", "pembalut", "rokok", "cigarette", "pulsa", "voucher", "token", "parfum",
	"lotion", "kosmetik", "makeup", "skincare",
}

func isNonFood(name string) bool {
	lower := strings.ToLower(name)
	for _, keyword := range nonFoodKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func normalizeCategory(value any) string {
	category, _ := value.(string)
	category = strings.ToLower(strings.TrimSpace(category))
	switch category {
	case "makanan", "food", "meal", "meals", "snack", "snacks", "cemilan", "camilan", "makanan ringan", "makanan kemasan", "produk makanan", "lauk", "nasi", "roti":
		return "makanan"
	case "minuman", "drink", "drinks", "beverage", "beverages", "minum", "minuman kemasan", "produk minuman":
		return "minuman"
	case "buah", "buah-buahan", "buah segar", "fruit", "fruits", "sayur", "vegetable":
		return "buah"
	case "":
		return ""
	default:
		return category
	}
}

func normalizeScanItems(result map[string]any) []map[string]any {
	items := []map[string]any{}
	collect := func(entry any) {
		item, ok := entry.(map[string]any)
		if !ok {
			return
		}
		name, _ := item["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" || isNonFood(name) {
			return
		}
		item["name"] = name
		if category, ok := item["category"].(string); ok && strings.TrimSpace(category) != "" {
			category = normalizeCategory(category)
			if category == "makanan" || category == "minuman" || category == "buah" {
				item["category"] = category
			} else {
				delete(item, "category")
			}
		}
		for _, key := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
			item[key] = toFloatOrNil(item[key])
		}
		items = append(items, item)
	}
	if list, ok := result["items"].([]any); ok {
		for _, entry := range list {
			collect(entry)
		}
		return items
	}
	if list, ok := result["items"].([]map[string]any); ok {
		for _, entry := range list {
			collect(entry)
		}
		return items
	}
	item := map[string]any{}
	for _, key := range []string{"name", "category", "serving_size", "calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg", "note", "mode", "user_id"} {
		if value, ok := result[key]; ok {
			item[key] = value
		}
	}
	if name, _ := item["name"].(string); strings.TrimSpace(name) != "" {
		for _, key := range []string{"calories", "protein_g", "carbs_g", "fat_g", "sugar_g", "sodium_mg"} {
			item[key] = toFloatOrNil(item[key])
		}
		items = append(items, item)
	}
	return items
}

func stripJSONFence(text string) string {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```JSON")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	trimmed = strings.TrimSpace(trimmed)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return trimmed
	}
	startObj, startArr := strings.Index(trimmed, "{"), strings.Index(trimmed, "[")
	start := -1
	isArray := false
	if startObj >= 0 && (startArr < 0 || startObj < startArr) {
		start = startObj
	} else if startArr >= 0 {
		start = startArr
		isArray = true
	}
	if start < 0 {
		return trimmed
	}
	rest := trimmed[start:]
	end := strings.LastIndex(rest, "}")
	if isArray {
		end = strings.LastIndex(rest, "]")
	}
	if end < 0 {
		return trimmed
	}
	return strings.TrimSpace(rest[:end+1])
}

func toFloatOrNil(value any) any {
	switch n := value.(type) {
	case nil:
		return nil
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return nil
		}
		return f
	case string:
		cleaned := strings.TrimSpace(n)
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' || r == '.' || r == '-' {
				return r
			}
			return -1
		}, cleaned)
		if digits == "" || digits == "-" || digits == "." {
			return nil
		}
		if f, err := strconv.ParseFloat(digits, 64); err == nil {
			return f
		}
		return nil
	default:
		return nil
	}
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
