package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

const endpoint = "https://api.groq.com/openai/v1/chat/completions"

type Client struct {
	APIKey  string
	APIKeys []string
	Model   string
	HTTP    *http.Client
	key     atomic.Uint64
}

func (c *Client) Generate(ctx context.Context, systemInstruction string, contents []any, jsonMode bool) (map[string]any, error) {
	messages := make([]map[string]any, 0, len(contents)+1)
	if systemInstruction != "" {
		messages = append(messages, map[string]any{"role": "system", "content": systemInstruction})
	}
	for _, raw := range contents {
		content, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("format konten AI tidak valid")
		}
		role, _ := content["role"].(string)
		if role == "model" {
			role = "assistant"
		}
		if role != "user" && role != "assistant" {
			return nil, errors.New("role konten AI tidak didukung")
		}
		parts, _ := content["parts"].([]any)
		var converted []any
		var text strings.Builder
		for _, rawPart := range parts {
			var part map[string]any
			switch value := rawPart.(type) {
			case map[string]any:
				part = value
			case map[string]string:
				part = map[string]any{"text": value["text"]}
			default:
				continue
			}
			if value, ok := part["text"].(string); ok {
				text.WriteString(value)
			}
			if inline, ok := part["inlineData"].(map[string]string); ok {
				mime, data := inline["mimeType"], inline["data"]
				if (mime == "image/jpeg" || mime == "image/png" || mime == "image/webp") && data != "" {
					converted = append(converted, map[string]any{
						"type":      "image_url",
						"image_url": map[string]string{"url": "data:" + mime + ";base64," + data},
					})
				}
			}
		}
		if text.Len() > 0 {
			if len(converted) == 0 {
				messages = append(messages, map[string]any{"role": role, "content": text.String()})
				continue
			}
			converted = append([]any{map[string]string{"type": "text", "text": text.String()}}, converted...)
		}
		if len(converted) == 0 {
			return nil, errors.New("konten pesan AI kosong")
		}
		messages = append(messages, map[string]any{"role": role, "content": converted})
	}
	payload := map[string]any{"model": c.Model, "messages": messages, "temperature": 0.2, "max_completion_tokens": 768, "include_reasoning": false}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.nextAPIKey())
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Groq HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data[:min(len(data), 2048)])))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("respons Groq tidak valid")
	}
	if result.Error != nil {
		return nil, fmt.Errorf("Groq: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return nil, errors.New("Groq tidak menghasilkan teks")
	}
	text := strings.TrimSpace(result.Choices[0].Message.Content)
	if !jsonMode {
		return map[string]any{"answer": text}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		return parsed, nil
	}
	var list []any
	if err := json.Unmarshal([]byte(text), &list); err == nil {
		return map[string]any{"items": list}, nil
	}
	return nil, errors.New("respons Groq bukan JSON valid")
}

func (c *Client) nextAPIKey() string {
	if len(c.APIKeys) == 0 {
		return c.APIKey
	}
	return c.APIKeys[(c.key.Add(1)-1)%uint64(len(c.APIKeys))]
}
