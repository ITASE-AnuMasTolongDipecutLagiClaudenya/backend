package turnstile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestVerify(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("secret") != "secret" || r.Form.Get("response") != "token" || r.Form.Get("remoteip") != "192.0.2.1" {
			t.Errorf("unexpected verification form: %v", url.Values(r.Form))
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	client := Client{Secret: "secret", HTTP: server.Client(), Endpoint: server.URL}
	valid, err := client.Verify(context.Background(), " token ", "192.0.2.1")
	if err != nil || !valid {
		t.Fatalf("Verify() = (%v, %v), want (true, nil)", valid, err)
	}
}
