package tools

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// WebTool supports fetch operations.
// Args: {"url": "https://..."}

type WebTool struct{}

func NewWebTool() *WebTool { return &WebTool{} }

func (t *WebTool) Name() string        { return "web" }
func (t *WebTool) Description() string { return "Fetch web content from a URL" }

func (t *WebTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to fetch (must be http or https)",
			},
		},
		"required": []string{"url"},
	}
}

func (t *WebTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	uStr, ok := args["url"].(string)
	if !ok || uStr == "" {
		return "", fmt.Errorf("web: 'url' argument required")
	}

	// Simple SSRF protection: reject localhost and common private IP ranges
	lower := strings.ToLower(uStr)
	if strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") || strings.Contains(lower, "::1") ||
		strings.Contains(lower, "10.") || strings.Contains(lower, "192.168.") || strings.Contains(lower, "172.16.") ||
		strings.Contains(lower, "169.254.") { // Link-local (AWS metadata, etc.)
		return "", fmt.Errorf("web: access to local or private network addresses is disallowed")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", uStr, nil)
	if err != nil {
		return "", err
	}
	// ... continue with request ...
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
