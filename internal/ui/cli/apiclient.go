package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// apiClient is a simple HTTP client for the Bootforge REST API.
type apiClient struct {
	base   string
	client *http.Client
}

func newAPIClient(base string) *apiClient {
	return &apiClient{
		base: base,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *apiClient) get(path string, v any) error {
	resp, err := c.client.Get(c.base + path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *apiClient) post(path string, v any) error {
	resp, err := c.client.Post(c.base+path, "application/json", nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}
