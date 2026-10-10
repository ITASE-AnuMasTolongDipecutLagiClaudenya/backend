package httpx

import (
	"encoding/json"
	"io"
	"net/http"
)

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func DecodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(target); err != nil {
		WriteError(w, 400, "Body JSON tidak valid")
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		WriteError(w, 400, "Body harus berisi satu objek JSON")
		return false
	}
	return true
}
