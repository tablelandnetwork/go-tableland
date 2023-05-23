package impl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
)

// webhookTemplate is the template used to generate the webhook content.
const contentTemplate = `
{{ if .Error }}
**Error processing Tableland event:**

Chain ID: {{ .ChainID }}
Block number: {{ .BlockNumber }}
Transaction hash: {{ .TxnHash }}
Table IDs: {{ .TableIDs }}
Error: **{{ .Error }}**
Error event index: {{ .ErrorEventIdx }}

{{ else }}
**Tableland event processed successfully:**

Chain ID: {{ .ChainID }}
Block number: {{ .BlockNumber }}
Transaction hash: {{ .TxnHash }}
Table IDs: {{ .TableIDs }}

{{ end }}
`

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

// Content function to return the formatted content for the webhook.
func content(r eventprocessor.Receipt) (string, error) {
	var c bytes.Buffer
	tmpl, err := template.New("content").Parse(contentTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}
	err = tmpl.Execute(&c, r)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}
	return c.String(), nil
}

// Webhook interface for sending webhooks to different services such as IFTTT or Discord etc.
type Webhook interface {
	Send(ctx context.Context, content eventprocessor.Receipt) error
}

// DiscordWebhook struct.
type DiscordWebhook struct {
	// URL is the webhook URL.
	URL string

	// WHData represents the webhook data that
	// should be JSON marshaled before sending as the POST body.
	WHData struct {
		Content string `json:"content"`
	}
}

// Send method formats the receipt as Webhook Data for Discord and Sends it.
func (w *DiscordWebhook) Send(ctx context.Context, r eventprocessor.Receipt) error {
	whContent, err := content(r)
	if err != nil {
		return fmt.Errorf("failed to get webhook content: %v", err)
	}
	// Discord requres that the data should be placed in the "content" field.
	w.WHData.Content = whContent
	return sendWebhookRequest(ctx, w.URL, w.WHData)
}

// NewWebhook function to create a new webhook.
func NewWebhook(urlStr string) (Webhook, error) {
	urlObject, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook url: %s", err)
	}

	if urlObject.Hostname() == "discord.com" {
		return &DiscordWebhook{
			URL: urlObject.String(),
		}, nil
	}

	return nil, fmt.Errorf("invalid webhook url")
}
