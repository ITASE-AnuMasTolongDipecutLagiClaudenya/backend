package exa

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSearchSendsHighlightsAndParsesResults(t *testing.T) {
	var gotPath, gotKey string
	var gotBody string
	c := &Client{
		APIKey:     "test-key",
		NumResults: 3,
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath, gotKey = r.URL.Path, r.Header.Get("x-api-key")
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"results":[{"title":"Kanzler","url":"https://fatsecret.co.id/k","highlights":["Kal 90"]}]}`)),
				Header:     make(http.Header), Request: r,
			}, nil
		})},
	}
	results, err := c.Search(context.Background(), "Kanzler Bakso nilai gizi")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search" || gotKey != "test-key" {
		t.Fatalf("path/key = %q/%q", gotPath, gotKey)
	}
	if !strings.Contains(gotBody, `"highlights":true`) || !strings.Contains(gotBody, `"numResults":3`) {
		t.Fatalf("body missing contents/numResults: %s", gotBody)
	}
	if len(results) != 1 || results[0].URL != "https://fatsecret.co.id/k" || len(results[0].Highlights) != 1 {
		t.Fatalf("results = %#v", results)
	}
}

func TestSearchRequiresKey(t *testing.T) {
	c := &Client{}
	if _, err := c.Search(context.Background(), "x"); err == nil {
		t.Fatal("search without key must fail")
	}
	if c.Configured() {
		t.Fatal("Configured should be false without key")
	}
}

func TestFilterTrustedRejectsHTTPAndUntrusted(t *testing.T) {
	results := []Result{
		{URL: "http://fatsecret.co.id/x"},
		{URL: "https://fatsecret.co.id/k"},
		{URL: "https://randomblog.com/nutrition"},
		{URL: "https://id.openfoodfacts.org/produk/1"},
	}
	got := filterTrusted(results, []string{"fatsecret.co.id", "id.openfoodfacts.org"})
	if len(got) != 2 {
		t.Fatalf("expected 2 trusted https results, got %d: %#v", len(got), got)
	}
	for _, r := range got {
		if r.URL[:8] != "https://" {
			t.Fatalf("non-https kept: %s", r.URL)
		}
	}
}

func TestFilterTrustedWithoutAllowlistKeepsAnyHTTPS(t *testing.T) {
	results := []Result{{URL: "http://a.com"}, {URL: "https://b.com"}}
	got := filterTrusted(results, nil)
	if len(got) != 1 || got[0].URL != "https://b.com" {
		t.Fatalf("expected https-only fallback, got %#v", got)
	}
}

func TestHostAllowedWildcard(t *testing.T) {
	if !hostAllowed("gizi.kemenkes.go.id", []string{"*.go.id"}) {
		t.Fatal("wildcard subdomain should match")
	}
	if hostAllowed("evil.com", []string{"*.go.id"}) {
		t.Fatal("unrelated host must not match")
	}
	if !hostAllowed("fatsecret.co.id", []string{"fatsecret.co.id"}) {
		t.Fatal("exact host should match")
	}
}
