package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/kr0nicas/picobot/internal/chat"
)

// StartTelegram is a convenience wrapper that uses the real polling implementation
// with the standard Telegram base URL.
// allowFrom is a list of Telegram user IDs permitted to interact with the bot.
// If empty, ALL users are allowed (open mode).
func StartTelegram(ctx context.Context, hub *chat.Hub, token string, allowFrom []string) error {
	if token == "" {
		return fmt.Errorf("telegram token not provided")
	}
	base := "https://api.telegram.org/bot" + token
	return StartTelegramWithBase(ctx, hub, token, base, allowFrom)
}

// StartTelegramWithBase starts long-polling against the given base URL (e.g., https://api.telegram.org/bot<TOKEN> or a test server URL).
// allowFrom restricts which Telegram user IDs may send messages. Empty means allow all.
func StartTelegramWithBase(ctx context.Context, hub *chat.Hub, token, base string, allowFrom []string) error {
	if base == "" {
		return fmt.Errorf("base URL is required")
	}

	// Build a fast lookup set for allowed user IDs.
	allowed := make(map[string]struct{}, len(allowFrom))
	for _, id := range allowFrom {
		allowed[id] = struct{}{}
	}

	client := &http.Client{Timeout: 45 * time.Second}

	// inbound polling goroutine
	go func() {
		log.Printf("telegram: starting inbound polling (allowFrom: %v)", allowFrom)
		offset := int64(0)
		for {
			select {
			case <-ctx.Done():
				log.Println("telegram: stopping inbound polling")
				return
			default:
			}

			values := url.Values{}
			values.Set("offset", strconv.FormatInt(offset, 10))
			values.Set("timeout", "30")
			u := base + "/getUpdates"
			resp, err := client.PostForm(u, values)
			if err != nil {
				log.Printf("telegram getUpdates error: %v", err)
				time.Sleep(5 * time.Second) // Wait bit longer on error
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var gu struct {
				Ok     bool `json:"ok"`
				Result []struct {
					UpdateID int64 `json:"update_id"`
					Message  *struct {
						MessageID int64 `json:"message_id"`
						From      *struct {
							ID int64 `json:"id"`
						} `json:"from"`
						Chat struct {
							ID int64 `json:"id"`
						} `json:"chat"`
						Text string `json:"text"`
					} `json:"message"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &gu); err != nil {
				log.Printf("telegram: invalid getUpdates response (len=%d): %v", len(body), err)
				time.Sleep(2 * time.Second)
				continue
			}
			for _, upd := range gu.Result {
				if upd.UpdateID >= offset {
					offset = upd.UpdateID + 1
				}
				if upd.Message == nil {
					continue
				}
				m := upd.Message
				fromID := ""
				if m.From != nil {
					fromID = strconv.FormatInt(m.From.ID, 10)
				}
				// Enforce allowFrom: if the list is empty, we drop all messages for security
				if len(allowed) == 0 {
					log.Printf("telegram: dropping message from user %s: no authorized users configured in allowFrom", fromID)
					continue
				}
				if _, ok := allowed[fromID]; !ok {
					log.Printf("telegram: dropping message from unauthorized user %s", fromID)
					continue
				}
				chatID := strconv.FormatInt(m.Chat.ID, 10)
				log.Printf("telegram: received message from %s, routing to hub", fromID)
				hub.In <- chat.Inbound{
					Channel:   "telegram",
					SenderID:  fromID,
					ChatID:    chatID,
					Content:   m.Text,
					Timestamp: time.Now(),
				}
			}
		}
	}()

	// outbound sender goroutine
	go func() {
		log.Println("telegram: starting outbound sender")
		client := &http.Client{Timeout: 15 * time.Second}
		for {
			select {
			case <-ctx.Done():
				log.Println("telegram: stopping outbound sender")
				return
			case out := <-hub.Out:
				if out.Channel != "telegram" {
					continue
				}
				log.Printf("telegram: sending message to chat %s", out.ChatID)
				u := base + "/sendMessage"
				chunks := splitMessage(out.Content, 4096)
				for _, chunk := range chunks {
					v := url.Values{}
					v.Set("chat_id", out.ChatID)
					v.Set("text", chunk)
					resp, err := client.PostForm(u, v)
					if err != nil {
						log.Printf("telegram sendMessage error: %v", err)
						break
					}
					respBody, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					if resp.StatusCode != 200 {
						log.Printf("telegram sendMessage non-200: %s body=%s", resp.Status, string(respBody))
						break
					}
				}
			}
		}
	}()

	return nil
}

// splitMessage splits text into chunks of at most maxLen characters,
// breaking at newlines when possible to keep messages readable.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		// Try to split at the last newline within the limit
		cut := maxLen
		if idx := lastIndexByte(text[:maxLen], '\n'); idx > 0 {
			cut = idx + 1
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}
	return chunks
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
