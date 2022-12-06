package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

type receiptConfig struct {
	timeout *time.Duration
}

// ReceiptOption controls the behavior of calls to Receipt.
type ReceiptOption func(*receiptConfig)

// WaitFor causes calls to Receipt to wait for the specified duration.
func WaitFor(timeout time.Duration) ReceiptOption {
	return func(rc *receiptConfig) {
		rc.timeout = &timeout
	}
}

// Receipt gets a transaction receipt.
func (c *Client) Receipt(
	ctx context.Context,
	txnHash string,
	options ...ReceiptOption,
) (*apiv1.TransactionReceipt, bool, error) {
	config := receiptConfig{}
	for _, option := range options {
		option(&config)
	}
	if config.timeout != nil {
		return c.waitForReceipt(ctx, txnHash, *config.timeout)
	}
	return c.getReceipt(ctx, txnHash)
}

func (c *Client) getReceipt(ctx context.Context, txnHash string) (*apiv1.TransactionReceipt, bool, error) {
	url := (*c.baseURL).
		JoinPath("api/v1/receipt").
		JoinPath(strconv.FormatInt(int64(c.chain.ID), 10)).
		JoinPath(txnHash)

	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("creating request: %s", err)
	}
	response, err := c.tblHTTP.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("calling get receipt by transaction hash: %s", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if response.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(response.Body)
		return nil, false, fmt.Errorf("failed call (status: %d, body: %s)", response.StatusCode, msg)
	}
	var tr apiv1.TransactionReceipt
	if err := json.NewDecoder(response.Body).Decode(&tr); err != nil {
		return nil, false, fmt.Errorf("unmarshaling result: %s", err)
	}
	return &tr, true, nil
}

func (c *Client) waitForReceipt(
	ctx context.Context,
	txnHash string,
	timeout time.Duration,
) (*apiv1.TransactionReceipt, bool, error) {
	for stay, timeout := true, time.After(timeout); stay; {
		select {
		case <-timeout:
			stay = false
		default:
			receipt, found, err := c.getReceipt(ctx, txnHash)
			if err != nil {
				return nil, false, err
			}
			if found {
				return receipt, found, nil
			}
			time.Sleep(time.Second)
		}
	}
	return nil, false, nil
}
