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

	ethChain := chains[1]
	filChain := chains[314]
	filCalibChain := chains[314159]

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
				fmt.Sprintf(
					"\n\n**Tableland event processed successfully:**\n\n"+
						"Chain ID: [1](%s)\n"+
						"Block number: 1\n"+
						"Transaction hash: [hash1](%s)\n"+
						"Table IDs: [0](%s)\n\n\n",
					ethChain.TBLDocsURL,
					ethChain.BlockExplorerURL+"/tx/hash1",
					ethChain.BlockExplorerURL+"/nft/"+ethChain.ContractAddr.String()+"/0",
				),
			},
		},
		{
			name: "single receipt Filecoin",
			receipts: []eventprocessor.Receipt{
				{
					BlockNumber:   1,
					ChainID:       314,
					TxnHash:       "hash1",
					Error:         nil,
					ErrorEventIdx: nil,
					TableIDs:      mockTableIDs[:1],
				},
			},
			expectedOutput: []string{
				fmt.Sprintf(
					"\n\n**Tableland event processed successfully:**\n\n"+
						"Chain ID: [314](%s)\n"+
						"Block number: 1\n"+
						"Transaction hash: [hash1](%s)\n"+
						"Table IDs: 0\n\n\n",
					filChain.TBLDocsURL,
					filChain.BlockExplorerURL+"/tx/hash1",
				),
			},
		},
		{
			name: "single receipt Filecoin Testnet",
			receipts: []eventprocessor.Receipt{
				{
					BlockNumber:   1,
					ChainID:       314159,
					TxnHash:       "hash1",
					Error:         nil,
					ErrorEventIdx: nil,
					TableIDs:      mockTableIDs[:1],
				},
			},
			expectedOutput: []string{
				fmt.Sprintf(
					"\n\n**Tableland event processed successfully:**\n\n"+
						"Chain ID: [314159](%s)\n"+
						"Block number: 1\n"+
						"Transaction hash: [hash1](%s)\n"+
						"Table IDs: 0\n\n\n",
					filCalibChain.TBLDocsURL,
					filCalibChain.BlockExplorerURL+"/tx/hash1",
				),
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
				fmt.Sprintf(
					"\n\n**Tableland event processed successfully:**\n\n"+
						"Chain ID: [1](%s)\n"+
						"Block number: 1\n"+
						"Transaction hash: [hash1](%s)\n"+
						"Table IDs: [0](%s), [1](%s)\n\n\n",
					ethChain.TBLDocsURL,
					ethChain.BlockExplorerURL+"/tx/hash1",
					ethChain.BlockExplorerURL+"/nft/"+ethChain.ContractAddr.String()+"/0",
					ethChain.BlockExplorerURL+"/nft/"+ethChain.ContractAddr.String()+"/1",
				),
				fmt.Sprintf(
					"\n\n**Tableland event processed successfully:**\n\n"+
						"Chain ID: [1](%s)\n"+
						"Block number: 2\n"+
						"Transaction hash: [hash2](%s)\n"+
						"Table IDs: [0](%s), [1](%s), [2](%s)\n\n\n",
					ethChain.TBLDocsURL,
					ethChain.BlockExplorerURL+"/tx/hash2",
					ethChain.BlockExplorerURL+"/nft/"+ethChain.ContractAddr.String()+"/0",
					ethChain.BlockExplorerURL+"/nft/"+ethChain.ContractAddr.String()+"/1",
					ethChain.BlockExplorerURL+"/nft/"+ethChain.ContractAddr.String()+"/2",
				),
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
				fmt.Sprintf(
					"\n\n**Error processing Tableland event:**\n\n"+
						"Chain ID: [1](%s)\n"+
						"Block number: 1\n"+
						"Transaction hash: [hash1](%s)\n"+
						"Table IDs: [0](%s)\n"+
						"Error: **error1**\n"+
						"Error event index: 1\n\n\n",
					ethChain.TBLDocsURL,
					ethChain.BlockExplorerURL+"/tx/hash1",
					ethChain.BlockExplorerURL+"/nft/"+ethChain.ContractAddr.String()+"/0",
				),
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
func (m *mockWebhook) Send(_ context.Context, r eventprocessor.Receipt) error {
	whContent, err := content(r)
	if err != nil {
		return fmt.Errorf("error creating webhook content: %v", err)
	}
	m.ch <- whContent
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
	discordWebhook, err := NewWebhook("https://discord.com/api/webhooks/1234567890/abcdefg")
	assert.NoError(t, err)
	assert.IsType(t, &DiscordWebhook{}, discordWebhook)

	// Test invalid webhook
	invalidWebhook, err := NewWebhook("https://example.com/webhook")
	assert.Error(t, err)
	assert.Nil(t, invalidWebhook)
}
