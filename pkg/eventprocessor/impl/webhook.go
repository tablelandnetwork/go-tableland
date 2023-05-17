package impl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	logger "github.com/rs/zerolog/log"
)

type webhookContent struct {
	Content string `json:"content"`
}

var webhookLogger = logger.With().Str("component", "webhook").Logger()

// Common function to send the webhook request.
func sendWebhookRequest(ctx context.Context, url string, body interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	postData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling webhook JSON: %s", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(postData))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing webhook: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			webhookLogger.Error().Err(err).Msg("closing")
		}
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook request failed with status code: %d", resp.StatusCode)
	}

	return nil
}

// Webhook interface for sending webhooks to different services such as IFTTT or Discord etc.
type Webhook interface {
	Send(ctx context.Context, content string) error
}

// DiscordWebhook struct.
type DiscordWebhook struct {
	URL string
}

// Send method for DiscordWebhook.
func (w *DiscordWebhook) Send(ctx context.Context, content string) error {
	return sendWebhookRequest(ctx, w.URL, webhookContent{
		Content: content,
	})
}

// NewWebhook function to create a new webhook.
func NewWebhook(endpointType string, url string) (Webhook, error) {
	if endpointType == "discord" {
		return &DiscordWebhook{
			URL: url,
		}, nil
	}

	return nil, fmt.Errorf("invalid webhook url")
}
