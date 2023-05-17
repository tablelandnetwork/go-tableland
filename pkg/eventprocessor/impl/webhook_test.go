package impl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/tables"
)

func TestExecuteWebhook(t *testing.T) {
	mockTableIDs := []tables.TableID{}
	for i := 0; i < 10; i++ {
		tID, err := tables.NewTableID(fmt.Sprintf("%d", i))
		if err != nil {
			t.Fatal(err)
		}
		mockTableIDs = append(mockTableIDs, tID)
	}

	testCases := []struct {
		name           string
		receipts       []eventprocessor.Receipt
		expectedOutput []string
	}{
		{
			name: "single receipt",
			receipts: []eventprocessor.Receipt{
				{
					BlockNumber:   1,
					ChainID:       1,
					TxnHash:       "hash1",
					Error:         nil,
					ErrorEventIdx: nil,
					TableIDs:      mockTableIDs[:1],
				},
			},
			expectedOutput: []string{
				"**Tableland event processed successfully:**\n\n" +
					"Chain ID: 1\n" +
					"Block number: 1\n" +
					"Transaction hash: hash1\n" +
					"Table IDs: 0\n\n",
			},
		},
		{
			name: "multiple receipts",
			receipts: []eventprocessor.Receipt{
				{
					BlockNumber:   1,
					ChainID:       1,
					TxnHash:       "hash1",
					Error:         nil,
					ErrorEventIdx: nil,
					TableIDs:      mockTableIDs[:2],
				},
				{
					BlockNumber:   2,
					ChainID:       1,
					TxnHash:       "hash2",
					Error:         nil,
					ErrorEventIdx: nil,
					TableIDs:      mockTableIDs[:3],
				},
			},
			expectedOutput: []string{
				"**Tableland event processed successfully:**\n\n" +
					"Chain ID: 1\n" +
					"Block number: 1\n" +
					"Transaction hash: hash1\n" +
					"Table IDs: 0,1\n\n",
				"**Tableland event processed successfully:**\n\n" +
					"Chain ID: 1\n" +
					"Block number: 2\n" +
					"Transaction hash: hash2\n" +
					"Table IDs: 0,1,2\n\n",
			},
		},
		{
			name: "receipt with error",
			receipts: []eventprocessor.Receipt{
				{
					BlockNumber:   1,
					ChainID:       1,
					TxnHash:       "hash1",
					TableIDs:      mockTableIDs[:1],
					Error:         &[]string{"error1"}[0],
					ErrorEventIdx: &[]int{1}[0],
				},
			},
			expectedOutput: []string{
				"**Error processing Tableland event:**\n\n" +
					"Chain ID: 1\n" +
					"Block number: 1\n" +
					"Transaction hash: hash1\n" +
					"Table IDs: 0\n" +
					"Error: **error1**\n" +
					"Error event index: 1\n\n",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new EventProcessor with a mock webhook
			ep := &EventProcessor{
				chainID: 1,
				webhook: &mockWebhook{
					ch: make(chan string, 10),
				},
			}

			// Call executeWebhook with the test data
			ep.executeWebhook(context.Background(), tc.receipts)

			// executeWebhook spins up several goroutines to fire webhooks concurrently
			// in these test case, our mocked implementation of `send` method will
			// write the webhook content to a channel instead of sending it to a real webhook
			// here can we read the content from the channel and assert that it is correct
			for i := 0; i < len(tc.receipts); i++ {
				content := <-ep.webhook.(*mockWebhook).ch
				ep.webhook.(*mockWebhook).content = append(ep.webhook.(*mockWebhook).content, content)
			}

			// Assert that the mock webhook received the correct content
			actualOutput := ep.webhook.(*mockWebhook).content
			assert.ElementsMatch(t, tc.expectedOutput, actualOutput)
		})
	}
}

type mockWebhook struct {
	content []string
	URL     string
	ch      chan string
}

// Send writes the webhook content to a channel instead of sending it to a real webhook.
func (m *mockWebhook) Send(_ context.Context, content string) error {
	m.ch <- content
	return nil
}

func TestSendWebhookRequest(t *testing.T) {
	// Create a new test server with a handler that echoes the request body
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}))

	// Make sure to close the test server when the test is done
	defer ts.Close()

	// Create a new webhook with the test server URL
	webhook := &mockWebhook{
		URL: ts.URL,
	}

	// Create a new request with a JSON payload
	payload := map[string]string{
		"foo": "bar",
	}

	// Send the request using the sendWebhookRequest function
	err := sendWebhookRequest(context.Background(), webhook.URL, payload)

	// Assert that the request was successful
	assert.NoError(t, err)
}

func TestNewWebhook(t *testing.T) {
	// Test Discord webhook
	discordWebhook, err := NewWebhook("discord", "https://discord.com/api/webhooks/1234567890/abcdefg")
	assert.NoError(t, err)
	assert.IsType(t, &DiscordWebhook{}, discordWebhook)

	// Test invalid webhook
	invalidWebhook, err := NewWebhook("invalid", "https://example.com/webhook")
	assert.Error(t, err)
	assert.Nil(t, invalidWebhook)
}
