package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) ValidateUser(ctx context.Context, token string) (string, string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/auth/v1/user", nil)
	req.Header.Set("apikey", c.AnonKey)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var user struct{ ID, Email string }
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user) != nil || user.ID == "" {
		return "", "", fmt.Errorf("invalid session")
	}
	return user.ID, user.Email, nil
}
