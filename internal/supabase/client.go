package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	BaseURL string
	AnonKey string
	HTTP    *http.Client
}

func UserFilter(id string) string { return "user_id=eq." + url.QueryEscape(id) }

func (c *Client) Query(ctx context.Context, authHeader, method, path string, body ...any) ([]map[string]any, error) {
	var reader io.Reader
	if len(body) > 0 {
		encoded, err := json.Marshal(body[0])
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/rest/v1/"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
		if method == "POST" && strings.Contains(path, "on_conflict=") {
			req.Header.Set("Prefer", "resolution=merge-duplicates,return=representation")
		} else {
			req.Header.Set("Prefer", "return=representation")
		}
	} else if method == "DELETE" {
		req.Header.Set("Prefer", "return=minimal")
	}
	resp, err := c.HTTP.Do(req)
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
