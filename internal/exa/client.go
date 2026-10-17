// Package exa is a minimal client for Exa's /search endpoint.
//
// The scan pipeline uses it to retrieve real nutrition pages for a product.
// Exa returns result URLs and page highlights, so citations come from Exa's
// index instead of being written by the language model.
package exa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const endpoint = "https://api.exa.ai/search"

type Client struct {
	APIKey string
	HTTP   *http.Client
	// NumResults caps how many pages are fetched per query. Three or five keeps
	// the source list short and the token cost low.
	NumResults int
	// TrustedDomains is a hard allowlist of nutrition sources. When set, only
	// these hosts are accepted; an empty list means HTTPS-only filtering.
	TrustedDomains []string
}

// Result is one retrieved page.
type Result struct {
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	Published  string   `json:"publishedDate"`
	Highlights []string `json:"highlights"`
}

// Configured reports whether an API key is present.
func (c *Client) Configured() bool { return strings.TrimSpace(c.APIKey) != "" }

// Search runs a single query and returns HTTPS results from trusted hosts.
//
// Sources are filtered after retrieval because Exa may return HTTP URLs or
// unrelated blogs; a citation shown to the user must be secure and come from a
// site that actually curates nutrition data.
func (c *Client) Search(ctx context.Context, query string) ([]Result, error) {
	if !c.Configured() {
		return nil, errors.New("EXA_API_KEY belum dikonfigurasi")
	}
	numResults := c.NumResults
	if numResults <= 0 {
		numResults = 3
	}

	// Prefer an allowlisted search. If the allowlist yields nothing for a
	// product, fall back once to an unfiltered search and keep only HTTPS
	// hosts, so niche products still get a source.
	results, err := c.search(ctx, query, numResults, c.TrustedDomains)
	if err != nil {
		return nil, err
	}
	filtered := filterTrusted(results, c.TrustedDomains)
	if len(filtered) == 0 && len(c.TrustedDomains) > 0 {
		results, err = c.search(ctx, query, numResults, nil)
		if err != nil {
			return nil, err
		}
		filtered = filterTrusted(results, nil)
	}
	return filtered, nil
}

func (c *Client) search(ctx context.Context, query string, numResults int, domains []string) ([]Result, error) {
	payload := map[string]any{
		"query":      query,
		"type":       "auto",
		"numResults": numResults,
		"contents":   map[string]any{"highlights": true},
	}
	if len(domains) > 0 {
		payload["includeDomains"] = domains
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Exa HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data[:min(len(data), 2048)])))
	}
	var parsed struct {
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, errors.New("respons Exa tidak valid")
	}
	return parsed.Results, nil
}

// filterTrusted keeps HTTPS results whose host matches the allowlist (or any
// host when the allowlist is empty). Wildcards such as "*.go.id" are honored.
func filterTrusted(results []Result, domains []string) []Result {
	kept := make([]Result, 0, len(results))
	for _, r := range results {
		parsed, err := url.Parse(r.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			continue
		}
		if len(domains) > 0 && !hostAllowed(parsed.Hostname(), domains) {
			continue
		}
		kept = append(kept, r)
	}
	return kept
}

func hostAllowed(host string, domains []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if strings.HasPrefix(domain, "*.") {
			if strings.HasSuffix(host, strings.TrimPrefix(domain, "*")) {
				return true
			}
			continue
		}
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
