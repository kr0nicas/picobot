package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicProvider implements the LLMProvider interface for Anthropic's Messages API.
type AnthropicProvider struct {
	APIKey    string
	APIBase   string // e.g. https://api.anthropic.com/v1
	MaxTokens int
	Client    *http.Client
}

func NewAnthropicProvider(apiKey, apiBase string, timeoutSecs, maxTokens int) *AnthropicProvider {
	if apiBase == "" {
		apiBase = "https://api.anthropic.com/v1"
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 60
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return &AnthropicProvider{
		APIKey:    apiKey,
		APIBase:   strings.TrimRight(apiBase, "/"),
		MaxTokens: maxTokens,
		Client: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
	}
}

func (p *AnthropicProvider) GetDefaultModel() string { return "claude-3-5-sonnet-latest" }

// Anthropic API specific shapes
type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`          // for tool_use
	Name      string          `json:"name,omitempty"`        // for tool_use
	Input     json.RawMessage `json:"input,omitempty"`       // for tool_use
	ToolUseID string          `json:"tool_use_id,omitempty"` // for tool_result
	Content   string          `json:"content,omitempty"`     // for tool_result
	IsError   bool            `json:"is_error,omitempty"`    // for tool_result
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Role       string           `json:"role"`
	Content    []anthropicBlock `json:"content"`
	StopReason string           `json:"stop_reason"`
	Error      *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string) (LLMResponse, error) {
	if p.APIKey == "" {
		return LLMResponse{}, errors.New("Anthropic provider: API key is not configured")
	}
	if model == "" {
		model = p.GetDefaultModel()
	}

	var systemPrompt string
	var anthropicMsgs []anthropicMessage

	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}

		if m.Role == "tool" {
			// Anthropic uses role: "user" for tool results with type: "tool_result"
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role: "user",
				Content: []anthropicBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
			continue
		}

		msgBlocks := []anthropicBlock{}
		if m.Content != "" {
			msgBlocks = append(msgBlocks, anthropicBlock{Type: "text", Text: m.Content})
		}

		for _, tc := range m.ToolCalls {
			args, _ := json.Marshal(tc.Arguments)
			msgBlocks = append(msgBlocks, anthropicBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Name,
				Input: args,
			})
		}

		anthropicMsgs = append(anthropicMsgs, anthropicMessage{
			Role:    m.Role,
			Content: msgBlocks,
		})
	}

	reqBody := anthropicRequest{
		Model:     model,
		Messages:  anthropicMsgs,
		System:    systemPrompt,
		MaxTokens: p.MaxTokens,
	}

	if len(tools) > 0 {
		for _, t := range tools {
			reqBody.Tools = append(reqBody.Tools, anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			})
		}
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return LLMResponse{}, err
	}

	apiURL := fmt.Sprintf("%s/messages", p.APIBase)
	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", p.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		return req, nil
	}

	resp, err := doWithRetry(ctx, p.Client, buildReq)
	if err != nil {
		return LLMResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return LLMResponse{}, fmt.Errorf("Anthropic API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var out anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return LLMResponse{}, err
	}

	if out.Error != nil {
		return LLMResponse{}, fmt.Errorf("Anthropic API error: %s - %s", out.Error.Type, out.Error.Message)
	}

	var finalContent strings.Builder
	var tcs []ToolCall
	hasToolCalls := false

	for _, block := range out.Content {
		if block.Type == "text" {
			finalContent.WriteString(block.Text)
		} else if block.Type == "tool_use" {
			hasToolCalls = true
			var args map[string]interface{}
			_ = json.Unmarshal(block.Input, &args)
			tcs = append(tcs, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return LLMResponse{
		Content:      strings.TrimSpace(finalContent.String()),
		HasToolCalls: hasToolCalls,
		ToolCalls:    tcs,
	}, nil
}
