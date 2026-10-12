package gemini

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
	"time"
)

type Client struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

func (c *Client) Generate(ctx context.Context, systemInstruction string, contents []any, jsonMode bool) (map[string]any, error) {
	payload := map[string]any{"contents": contents, "generationConfig": map[string]any{"temperature": 0.2}}
	if systemInstruction != "" {
		payload["systemInstruction"] = map[string]any{"parts": []any{map[string]string{"text": systemInstruction}}}
	}
	if jsonMode {
		payload["generationConfig"].(map[string]any)["responseMimeType"] = "application/json"
	}
	body, _ := json.Marshal(payload)
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(c.Model) + ":generateContent"
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", c.APIKey)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if (resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests) && attempt < 2 {
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
		text, err := Text(data)
		if err != nil {
			return nil, err
		}
		if jsonMode {
			cleaned := StripFence(text)
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

func Text(data []byte) (string, error) {
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

func StripFence(text string) string {
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
