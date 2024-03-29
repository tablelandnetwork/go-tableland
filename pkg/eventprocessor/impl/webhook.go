package impl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/tables"
)

// webhookTemplate is the template used to generate the webhook content.
const contentTemplate = `
{{ if .Error }}
**Error processing Tableland event:**

Chain ID: [{{ .ChainID }}]({{ .TBLDocsURL }})
Block number: {{ .BlockNumber }}
Transaction hash: [{{ .TxnHash }}]({{ .TxnURL }})
Table IDs: {{ .TableIDs }}
Error: **{{ .Error }}**
Error event index: {{ .ErrorEventIdx }}

{{ else }}
**Tableland event processed successfully:**

Chain ID: [{{ .ChainID }}]({{ .TBLDocsURL }})
Block number: {{ .BlockNumber }}
Transaction hash: [{{ .TxnHash }}]({{ .TxnURL }})
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

type chain struct {
	ID               int64
	Name             string
	ContractAddr     common.Address
	TBLDocsURL       string
	BlockExplorerURL string
	SupportsNFTView  bool
}

var chains = map[tableland.ChainID]chain{
	// Ethereum
	1: {
		ContractAddr:     common.HexToAddress("0x012969f7e3439a9B04025b5a049EB9BAD82A8C12"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/ethereum",
		BlockExplorerURL: "https://etherscan.io",
		SupportsNFTView:  true,
	},
	// Optimism
	10: {
		ContractAddr:     common.HexToAddress("0xfad44BF5B843dE943a09D4f3E84949A11d3aa3e6"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/optimism",
		BlockExplorerURL: "https://optimistic.etherscan.io",
		SupportsNFTView:  false,
	},
	// Polygon
	137: {
		ContractAddr:     common.HexToAddress("0x5c4e6A9e5C1e1BF445A062006faF19EA6c49aFeA"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/polygon",
		BlockExplorerURL: "https://polygonscan.com",
		SupportsNFTView:  false,
	},
	// Arbitrum One
	42161: {
		ContractAddr:     common.HexToAddress("0x9aBd75E8640871A5a20d3B4eE6330a04c962aFfd"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/arbitrum",
		BlockExplorerURL: "https://arbiscan.io",
		SupportsNFTView:  false,
	},
	// Arbitrum Nova
	42170: {
		ContractAddr:     common.HexToAddress("0x1a22854c5b1642760a827f20137a67930ae108d2"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/arbitrum",
		BlockExplorerURL: "https://nova.arbiscan.io",
		SupportsNFTView:  false,
	},
	// Filecoin
	314: {
		ContractAddr:     common.HexToAddress("0x59EF8Bf2d6c102B4c42AEf9189e1a9F0ABfD652d"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/Filecoin",
		BlockExplorerURL: "https://filfox.info/en",
		SupportsNFTView:  false,
	},
	// Goerli
	5: {
		ContractAddr:     common.HexToAddress("0xDA8EA22d092307874f30A1F277D1388dca0BA97a"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/ethereum",
		BlockExplorerURL: "https://goerli.etherscan.io",
		SupportsNFTView:  true,
	},
	// Sepolia
	11155111: {
		ContractAddr:     common.HexToAddress("0xc50C62498448ACc8dBdE43DA77f8D5D2E2c7597D"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/ethereum",
		BlockExplorerURL: "https://sepolia.etherscan.io",
		SupportsNFTView:  true,
	},
	// Optimism Goerli
	420: {
		ContractAddr:     common.HexToAddress("0xC72E8a7Be04f2469f8C2dB3F1BdF69A7D516aBbA"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/optimism",
		BlockExplorerURL: "https://blockscout.com/optimism/goerli",
		SupportsNFTView:  false,
	},
	// Arbitrum Goerli
	421613: {
		ContractAddr:     common.HexToAddress("0x033f69e8d119205089Ab15D340F5b797732f646b"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/arbitrum",
		BlockExplorerURL: "https://goerli-rollup-explorer.arbitrum.io",
		SupportsNFTView:  false,
	},
	// Filecoin Calibration
	314159: {
		ContractAddr:     common.HexToAddress("0x030BCf3D50cad04c2e57391B12740982A9308621"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/Filecoin",
		BlockExplorerURL: "https://calibration.filfox.info/en",
		SupportsNFTView:  false,
	},
	// Polygon Mumbai
	80001: {
		ContractAddr:     common.HexToAddress("0x4b48841d4b32C4650E4ABc117A03FE8B51f38F68"),
		TBLDocsURL:       "https://docs.tableland.xyz/fundamentals/chains/polygon",
		BlockExplorerURL: "https://mumbai.polygonscan.com",
		SupportsNFTView:  false,
	},
}

type webhookContentData struct {
	ChainID       tableland.ChainID
	TBLDocsURL    string
	BlockNumber   int64
	TxnHash       string
	TxnURL        string
	TableIDs      string
	Error         *string
	ErrorEventIdx *int
}

// getTableNFTViews returns the NFT views for the given table IDs.
func getTableNFTViews(tableIDs tables.TableIDs, chainID tableland.ChainID) string {
	var tableNFTURLs []string
	ch := chains[chainID]
	for _, tableID := range tableIDs {
		if !ch.SupportsNFTView {
			tableNFTURLs = append(tableNFTURLs, tableID.String())
		} else {
			blockExplorerURL := ch.BlockExplorerURL
			contractAddr := ch.ContractAddr
			tableNFTURLs = append(tableNFTURLs,
				fmt.Sprintf("[%s](%s/nft/%s/%s)", tableID.String(),
					blockExplorerURL, contractAddr, tableID.String()))
		}
	}
	return strings.Join(tableNFTURLs, ", ")
}

// Content function to return the formatted content for the webhook.
func content(r eventprocessor.Receipt) (string, error) {
	var c bytes.Buffer
	tmpl, err := template.New("content").Parse(contentTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %v", err)
	}

	txnURL := chains[r.ChainID].BlockExplorerURL + "/tx/" + r.TxnHash
	docsURL := chains[r.ChainID].TBLDocsURL
	err = tmpl.Execute(&c, webhookContentData{
		ChainID:       r.ChainID,
		TBLDocsURL:    docsURL,
		BlockNumber:   r.BlockNumber,
		TxnHash:       r.TxnHash,
		TxnURL:        txnURL,
		TableIDs:      getTableNFTViews(r.TableIDs, r.ChainID),
		Error:         r.Error,
		ErrorEventIdx: r.ErrorEventIdx,
	})
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
