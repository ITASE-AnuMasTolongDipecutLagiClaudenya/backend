package handler

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"nutrily/backend/internal/httpx"
	"nutrily/backend/internal/middleware"
)

func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	if h.CloudinaryCloud == "" || h.CloudinaryKey == "" || h.CloudinarySecret == "" {
		httpx.WriteError(w, http.StatusServiceUnavailable, "Cloudinary belum dikonfigurasi")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20+1)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Foto tidak valid atau melebihi 5 MB")
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "File foto wajib diisi")
		return
	}
	defer file.Close()
	if header.Size > 5<<20 {
		httpx.WriteError(w, http.StatusBadRequest, "Ukuran foto maksimal 5 MB")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 5<<20+1))
	if err != nil || len(data) > 5<<20 {
		httpx.WriteError(w, http.StatusBadRequest, "Ukuran foto maksimal 5 MB")
		return
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Konten file bukan gambar yang valid")
		return
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" {
		httpx.WriteError(w, http.StatusBadRequest, "Gunakan foto JPG atau PNG")
		return
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	publicID := "avatar_" + u.ID
	params := "public_id=" + publicID + "&timestamp=" + timestamp
	signature := sha1.Sum([]byte(params + h.CloudinarySecret))
	form := url.Values{"api_key": {h.CloudinaryKey}, "timestamp": {timestamp}, "public_id": {publicID}, "signature": {hex.EncodeToString(signature[:])}}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, values := range form {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				httpx.WriteError(w, http.StatusBadGateway, "Gagal menyiapkan upload foto")
				return
			}
		}
	}
	part, err := writer.CreateFormFile("file", "avatar"+avatarExtension(contentType))
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "Gagal menyiapkan upload foto")
		return
	}
	if _, err = io.Copy(part, bytes.NewReader(data)); err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "Gagal menyiapkan upload foto")
		return
	}
	if err := writer.Close(); err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "Gagal menyiapkan upload foto")
		return
	}
	endpoint := "https://api.cloudinary.com/v1_1/" + url.PathEscape(h.CloudinaryCloud) + "/image/upload"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, body)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "Gagal menghubungi Cloudinary")
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "Gagal menghubungi Cloudinary")
		return
	}
	defer resp.Body.Close()
	result, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(result, &failure) == nil && failure.Error.Message != "" {
			log.Printf("Cloudinary avatar upload failed (%d): %s", resp.StatusCode, failure.Error.Message)
			httpx.WriteError(w, http.StatusBadGateway, "Cloudinary gagal menyimpan foto: "+failure.Error.Message)
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, "Cloudinary gagal menyimpan foto")
		return
	}
	var payload struct {
		SecureURL string `json:"secure_url"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || payload.SecureURL == "" {
		httpx.WriteError(w, http.StatusBadGateway, "Respons Cloudinary tidak valid")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"avatar_url": payload.SecureURL, "public_id": publicID})
}

func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request, u middleware.Identity) {
	if h.CloudinaryCloud == "" || h.CloudinaryKey == "" || h.CloudinarySecret == "" {
		httpx.WriteError(w, http.StatusServiceUnavailable, "Cloudinary belum dikonfigurasi")
		return
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	publicID := "avatar_" + u.ID
	params := "public_id=" + publicID + "&timestamp=" + timestamp
	signature := sha1.Sum([]byte(params + h.CloudinarySecret))
	form := url.Values{"api_key": {h.CloudinaryKey}, "timestamp": {timestamp}, "public_id": {publicID}, "signature": {hex.EncodeToString(signature[:])}}
	endpoint := "https://api.cloudinary.com/v1_1/" + url.PathEscape(h.CloudinaryCloud) + "/image/destroy"
	resp, err := http.PostForm(endpoint, form)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "Gagal menghubungi Cloudinary")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpx.WriteError(w, http.StatusBadGateway, "Cloudinary gagal menghapus foto")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func avatarExtension(contentType string) string {
	if contentType == "image/png" {
		return ".png"
	}
	return ".jpg"
}
