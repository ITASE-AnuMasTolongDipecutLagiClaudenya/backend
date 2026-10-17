package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type Client struct {
	APIKey  string
	APIKeys []string
	Model   string
	HTTP    *http.Client
	key     atomic.Uint64
}

func (c *Client) Generate(ctx context.Context, systemInstruction string, contents []any, jsonMode bool) (map[string]any, error) {
	return c.generate(ctx, systemInstruction, contents, jsonMode, false)
}

func (c *Client) GenerateGrounded(ctx context.Context, prompt string) (map[string]any, error) {
	contents := []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": prompt}}}}
	return c.generate(ctx, "", contents, false, true)
}

func (c *Client) generate(ctx context.Context, systemInstruction string, contents []any, jsonMode, googleSearch bool) (map[string]any, error) {
	generationConfig := map[string]any{"temperature": 0.2}
	if strings.HasPrefix(c.Model, "gemini-3") {
		generationConfig["thinkingConfig"] = map[string]any{"thinkingBudget": 0}
	}
	payload := map[string]any{"contents": contents, "generationConfig": generationConfig}
	if googleSearch {
		payload["tools"] = []any{map[string]any{"google_search": map[string]any{}}}
	}
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
		req.Header.Set("x-goog-api-key", c.nextAPIKey())
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil || attempt == 2 {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, err
			}
			timer := time.NewTimer(time.Duration(1<<attempt) * 500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
				continue
			}
		}
		if (resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusTooManyRequests) && attempt < 2 {
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
		if googleSearch {
			var scan map[string]any
			if err := json.Unmarshal([]byte(StripFence(text)), &scan); err != nil {
				return nil, errors.New("hasil estimasi Gemini bukan JSON valid")
			}
			applyGroundedSources(scan, data)
			return scan, nil
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

func applyGroundedSources(scan map[string]any, response []byte) {
	var result struct {
		Candidates []struct {
			GroundingMetadata struct {
				GroundingChunks []struct {
					Web struct {
						URI   string `json:"uri"`
						Title string `json:"title"`
					} `json:"web"`
				} `json:"groundingChunks"`
			} `json:"groundingMetadata"`
		} `json:"candidates"`
	}
	if json.Unmarshal(response, &result) != nil || len(result.Candidates) == 0 {
		return
	}
	allowed := make(map[string]map[string]string)
	for _, chunk := range result.Candidates[0].GroundingMetadata.GroundingChunks {
		parsed, err := url.Parse(chunk.Web.URI)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			continue
		}
		if len(allowed) < 5 {
			allowed[chunk.Web.URI] = map[string]string{"title": chunk.Web.Title, "url": chunk.Web.URI}
		}
	}
	items, ok := scan["items"].([]any)
	if !ok {
		return
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		urls, _ := item["source_urls"].([]any)
		delete(item, "source_urls")
		seen := make(map[string]bool)
		var sources []map[string]string
		for _, rawURL := range urls {
			value, ok := rawURL.(string)
			source, found := allowed[value]
			if !ok || !found || seen[value] {
				continue
			}
			seen[value] = true
			sources = append(sources, source)
		}
		item["sources"] = sources
		if len(sources) == 0 {
			for _, chunk := range result.Candidates[0].GroundingMetadata.GroundingChunks {
				if source, found := allowed[chunk.Web.URI]; found {
					if _, exists := seen[chunk.Web.URI]; exists {
						continue
					}
					item["sources"] = append(item["sources"].([]map[string]string), source)
					seen[chunk.Web.URI] = true
				}
			}
		}
	}
}

func (c *Client) GenerateImage(ctx context.Context, prompt string, data []byte, contentType string, jsonMode bool) (map[string]any, error) {
	contents := []any{map[string]any{"role": "user", "parts": []any{
		map[string]string{"text": prompt},
		map[string]any{"inlineData": map[string]string{"mimeType": contentType, "data": base64.StdEncoding.EncodeToString(data)}},
	}}}
	return c.Generate(ctx, "", contents, jsonMode)
}

func (c *Client) nextAPIKey() string {
	if len(c.APIKeys) == 0 {
		return c.APIKey
	}
	return c.APIKeys[(c.key.Add(1)-1)%uint64(len(c.APIKeys))]
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
