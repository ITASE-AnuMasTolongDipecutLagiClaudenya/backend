package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

const (
	chatEndpoint      = "https://api.openai.com/v1/chat/completions"
	responsesEndpoint = "https://api.openai.com/v1/responses"
)

// searchInstructions is a terse directive for the Responses API search step.
// The full cost-efficiency rules and output schema live in the input prompt
// (estimatePrompt) to avoid duplicating instructions and wasting input tokens.
const searchInstructions = `Balas hanya satu objek JSON valid sesuai skema di input. Tanpa Markdown, code fence, atau teks di luar JSON.`

// Client talks to the OpenAI REST API (chat completions for vision/OCR and
// the Responses API for web-search-grounded nutrition lookups). All methods
// are safe for concurrent use; API keys rotate round-robin across requests.
type Client struct {
	APIKey  string
	APIKeys []string
	Model   string
	HTTP    *http.Client
	key     atomic.Uint64
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) nextAPIKey() string {
	if len(c.APIKeys) == 0 {
		return c.APIKey
	}
	return c.APIKeys[(c.key.Add(1)-1)%uint64(len(c.APIKeys))]
}

func (c *Client) configured() bool {
	return c.APIKey != "" || len(c.APIKeys) > 0
}

// Configured reports whether an API key is present.
func (c *Client) Configured() bool {
	return c.configured()
}

// GenerateImage performs vision OCR: it sends prompt + image and returns the
// parsed JSON result. jsonMode requests a JSON object response.
func (c *Client) GenerateImage(ctx context.Context, prompt string, data []byte, contentType string, jsonMode bool) (map[string]any, error) {
	content := []any{
		map[string]string{"type": "text", "text": prompt},
		map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)}},
	}
	payload := map[string]any{
		"model":       c.Model,
		"messages":    []any{map[string]any{"role": "user", "content": content}},
		"temperature": 0.2,
		// OCR must emit every receipt line, and long receipts need headroom.
		// 500 tokens truncated real receipts mid-JSON (finish_reason=length).
		"max_completion_tokens": 2000,
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	return c.postChat(ctx, payload, jsonMode)
}

// GenerateSearch grounds nutrition lookups with web search. It returns a
// normalized {"items":[…]}: the model's returned items, with search citations
// extracted from the Responses API output.
func (c *Client) GenerateSearch(ctx context.Context, prompt string) (map[string]any, error) {
	payload := map[string]any{
		"model":             c.Model,
		"instructions":      searchInstructions,
		"input":             prompt,
		"max_output_tokens": 1500,
		"temperature":       0.2,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	result, err := c.do(ctx, http.MethodPost, responsesEndpoint, body, "application/json")
	if err != nil {
		return nil, err
	}
	return parseResponses(result)
}

func (c *Client) postChat(ctx context.Context, payload map[string]any, jsonMode bool) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	result, err := c.do(ctx, http.MethodPost, chatEndpoint, body, "application/json")
	if err != nil {
		return nil, err
	}
	return parseChatCompletion(result, jsonMode)
}

// do performs one request with at most one retry on transient failures.
func (c *Client) do(ctx context.Context, method, endpoint string, body []byte, contentType string) ([]byte, error) {
	if !c.configured() {
		return nil, errors.New("OPENAI_API_KEY belum dikonfigurasi")
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			// Single bounded backoff between the only two attempts.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.nextAPIKey())
		req.Header.Set("Content-Type", contentType)
		resp, err := c.httpClient().Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue // retry transient network failure
		}
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("OpenAI HTTP %d", resp.StatusCode)
			continue // retry transient provider failure
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("OpenAI HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data[:min(len(data), 2048)])))
		}
		return data, nil
	}
	return nil, lastErr
}

func parseChatCompletion(data []byte, jsonMode bool) (map[string]any, error) {
	var result struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("respons OpenAI tidak valid")
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return nil, errors.New("OpenAI tidak menghasilkan teks")
	}
	text := strings.TrimSpace(result.Choices[0].Message.Content)
	if !jsonMode {
		return map[string]any{"answer": text}, nil
	}
	if result.Choices[0].FinishReason == "length" {
		return nil, errors.New("hasil OCR terpotong karena melebihi batas token; coba foto dengan lebih sedikit item atau pecah menjadi beberapa foto")
	}
	return parseJSONText(text)
}

// parseResponses extracts the model's final text and its web-search citations
// from the Responses API output.
func parseResponses(data []byte) (map[string]any, error) {
	var result struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Annotations []struct {
					Type  string `json:"type"`
					URL   string `json:"url"`
					Title string `json:"title"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("respons pencarian OpenAI tidak valid")
	}
	var text strings.Builder
	var sources []map[string]string
	seen := map[string]bool{}
	for _, out := range result.Output {
		if out.Type == "web_search_call" {
			continue
		}
		if out.Type == "message" {
			for _, c := range out.Content {
				if c.Type == "output_text" {
					text.WriteString(c.Text)
				}
				for _, a := range c.Annotations {
					if a.Type != "url_citation" || a.URL == "" || seen[a.URL] {
						continue
					}
					seen[a.URL] = true
					sources = append(sources, map[string]string{"url": a.URL, "title": a.Title})
				}
			}
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return nil, errors.New("OpenAI tidak menghasilkan hasil pencarian")
	}
	parsed, err := parseJSONText(text.String())
	if err != nil {
		return nil, err
	}
	if len(sources) > 0 {
		parsed["_search_sources"] = sources
	}
	return parsed, nil
}

func parseJSONText(text string) (map[string]any, error) {
	cleaned := extractJSON(text)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(cleaned), &parsed); err == nil {
		return parsed, nil
	}
	var list []any
	if err := json.Unmarshal([]byte(cleaned), &list); err == nil {
		return map[string]any{"items": list}, nil
	}
	return nil, errors.New("respons OpenAI bukan JSON valid; tidak dicoba ulang")
}

// extractJSON pulls the JSON payload out of model text that may include a
// prose preamble and/or a ```json fence. Models occasionally preface the
// object even in JSON mode, which previously broke OCR parsing.
func extractJSON(text string) string {
	trimmed := strings.TrimSpace(text)
	// Drop a fenced block's opening line, if present.
	if idx := strings.Index(trimmed, "```"); idx >= 0 {
		rest := trimmed[idx+3:]
		rest = strings.TrimPrefix(rest, "json")
		rest = strings.TrimPrefix(rest, "JSON")
		rest = strings.TrimSpace(rest)
		if end := strings.LastIndex(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		if candidate := strings.TrimSpace(rest); candidate != "" {
			trimmed = candidate
		}
	}
	// If there is still leading prose, cut from the first object/array opener.
	if first := strings.IndexAny(trimmed, "{["); first > 0 {
		trimmed = trimmed[first:]
	}
	// Trim trailing prose after the final closer.
	if last := strings.LastIndexAny(trimmed, "}]"); last >= 0 && last < len(trimmed)-1 {
		trimmed = trimmed[:last+1]
	}
	return strings.TrimSpace(trimmed)
}
