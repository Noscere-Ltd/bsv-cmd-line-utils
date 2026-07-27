// Package utxo fetches unspent outputs for an address, using BananaBlocks as the
// primary source and WhatsOnChain as a fallback. Both expose the same
// /unspent/all response shape, so callers parse a single format.
package utxo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

// bananaMaxLimit is the largest page BananaBlocks will return; asking for more
// is a 400. Its default is only 100, so the limit is always sent explicitly.
const bananaMaxLimit = 1000

var (
	// ErrAPI is returned when every provider fails.
	ErrAPI = errors.New("UTXO API error")
	// errStatus is returned when a provider responds with a non-200 status.
	errStatus = errors.New("unexpected status")
	// errTruncated is returned when a provider returns a full page, meaning the
	// UTXO list may be incomplete.
	errTruncated = errors.New("result may be truncated")
)

// provider is one /unspent/all endpoint.
type provider struct {
	url string
	// resultCap is the most results this endpoint can return, or 0 if uncapped.
	// A response that fills the cap is treated as a failure so the next provider
	// is tried: a silently truncated UTXO list would leave satoshis behind.
	resultCap int
}

// unspentProviders returns the endpoints to try, in priority order.
//
// BananaBlocks selects the network by host, not by path: the path segment is
// fixed at /bsv/main on every deployment.
func unspentProviders(addr string, testnet bool) []provider {
	bananaHost, wocNetwork := "bananablocks.com", "main"
	if testnet {
		bananaHost, wocNetwork = "test.bananablocks.com", "test"
	}

	return []provider{
		{
			url:       fmt.Sprintf("https://%s/api/v1/bsv/main/address/%s/unspent/all?limit=%d", bananaHost, addr, bananaMaxLimit),
			resultCap: bananaMaxLimit,
		},
		{
			url: fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent/all", wocNetwork, addr),
		},
	}
}

// Fetch returns the raw /unspent/all response body for addr, falling back to the
// next provider whenever one errors, returns a non-200, or returns a result that
// may be truncated.
func Fetch(ctx context.Context, addr string, testnet, debug bool) ([]byte, error) {
	providers := unspentProviders(addr, testnet)
	errs := make([]error, 0, len(providers))

	for _, p := range providers {
		if debug {
			log.Printf("Fetching UTXOs from %s", p.url)
		}

		body, err := get(ctx, p.url)
		if err == nil {
			if err = checkComplete(body, p.resultCap); err == nil {
				return body, nil
			}
		}

		if debug {
			log.Printf("UTXO fetch failed: %v", err)
		}

		errs = append(errs, err)
	}

	return nil, fmt.Errorf("%w: %w", ErrAPI, errors.Join(errs...))
}

// checkComplete rejects a response that fills the provider's result cap, since
// the remaining UTXOs would be silently dropped.
func checkComplete(body []byte, resultCap int) error {
	if resultCap == 0 {
		return nil
	}

	var response struct {
		Result []json.RawMessage `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse UTXOs: %w", err)
	}

	if len(response.Result) >= resultCap {
		return fmt.Errorf("%w: hit the %d result cap", errTruncated, resultCap)
	}

	return nil
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read response: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %w %d: %s", url, errStatus, resp.StatusCode, string(body))
	}

	return body, nil
}
