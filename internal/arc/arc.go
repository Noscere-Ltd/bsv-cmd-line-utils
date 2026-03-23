// Package arc provides a client for interacting with ARC (BSV Transaction Processing) endpoints.
// ARC is the BSV blockchain's transaction processing and broadcasting service that handles
// transaction lifecycle management including validation, broadcasting, and status tracking.
//
// The package supports:
//   - Broadcasting raw transactions to the BSV network via ARC
//   - Checking transaction status and tracking transaction lifecycle
//   - Full ARC status enumeration (RECEIVED, STORED, ANNOUNCED_TO_NETWORK, SEEN_ON_NETWORK, MINED, etc.)
//   - Helper functions for status visualization and description
package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Transaction statuses based on ARC specification
const (
	StatusReceived           = "RECEIVED"
	StatusStored             = "STORED"
	StatusAnnouncedToNetwork = "ANNOUNCED_TO_NETWORK"
	StatusSeenOnNetwork      = "SEEN_ON_NETWORK"
	StatusSeenByNetwork      = "SEEN_BY_NETWORK"
	StatusMined              = "MINED"
	StatusRejected           = "REJECTED"
	StatusDoubleSpend        = "DOUBLE_SPEND_ATTEMPTED"
)

// ARCClient handles communication with ARC endpoints
type ARCClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// TransactionRequest represents a transaction broadcast request
type TransactionRequest struct {
	RawTx string `json:"rawTx"`
}

// TransactionResponse represents the response from ARC transaction submission
type TransactionResponse struct {
	TxID      string `json:"txid"`
	TxStatus  string `json:"txStatus"`
	ExtraInfo string `json:"extraInfo,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// TransactionStatus represents the status check response
type TransactionStatus struct {
	TxID        string `json:"txid"`
	TxStatus    string `json:"txStatus"`
	ExtraInfo   string `json:"extraInfo,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	BlockHash   string `json:"blockHash,omitempty"`
	BlockHeight int64  `json:"blockHeight,omitempty"`
}

// ErrorResponse represents an error response from ARC
type ErrorResponse struct {
	Status int    `json:"status"`
	Code   int    `json:"code"`
	Error  string `json:"error"`
}

// NewARCClient creates a new ARC client
func NewARCClient(baseURL, apiKey string) *ARCClient {
	return &ARCClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// BroadcastTransaction broadcasts a transaction to the ARC network
func (c *ARCClient) BroadcastTransaction(rawTx string) (*TransactionResponse, error) {
	url := c.baseURL + "/v1/tx"

	reqBody := TransactionRequest{
		RawTx: rawTx,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %w", resp.StatusCode, err)
		}
		if errorResp.Error == "" {
			return nil, fmt.Errorf("request failed with HTTP status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("ARC error: %s (HTTP %d, code: %d)", errorResp.Error, resp.StatusCode, errorResp.Code)
	}

	var txResp TransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&txResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &txResp, nil
}

// GetTransactionStatus checks the status of a transaction
func (c *ARCClient) GetTransactionStatus(txid string) (*TransactionStatus, error) {
	url := c.baseURL + "/v1/tx/" + txid

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %w", resp.StatusCode, err)
		}
		if errorResp.Error == "" {
			return nil, fmt.Errorf("request failed with HTTP status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("ARC error: %s (HTTP %d, code: %d)", errorResp.Error, resp.StatusCode, errorResp.Code)
	}

	var status TransactionStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

// IsTransactionFinal returns true if the transaction has reached a final state
func IsTransactionFinal(status string) bool {
	switch status {
	case StatusMined, StatusRejected, StatusDoubleSpend:
		return true
	default:
		return false
	}
}

// GetStatusColor returns a color code for the transaction status (for TUI styling)
func GetStatusColor(status string) string {
	switch status {
	case StatusReceived:
		return "#FFD700" // Gold
	case StatusStored:
		return "#FFA500" // Orange
	case StatusAnnouncedToNetwork:
		return "#1E90FF" // DodgerBlue
	case StatusSeenOnNetwork, StatusSeenByNetwork:
		return "#32CD32" // LimeGreen
	case StatusMined:
		return "#00FF00" // Green
	case StatusRejected:
		return "#FF0000" // Red
	case StatusDoubleSpend:
		return "#FF4500" // OrangeRed
	default:
		return "#FFFFFF" // White
	}
}

// GetStatusDescription returns a human-readable description of the status
func GetStatusDescription(status string) string {
	switch status {
	case StatusReceived:
		return "Transaction received by ARC"
	case StatusStored:
		return "Transaction stored and validated by ARC"
	case StatusAnnouncedToNetwork:
		return "Transaction announced to the BSV network"
	case StatusSeenOnNetwork, StatusSeenByNetwork:
		return "Transaction seen on the BSV network"
	case StatusMined:
		return "Transaction successfully mined in a block"
	case StatusRejected:
		return "Transaction rejected by the network"
	case StatusDoubleSpend:
		return "Transaction rejected due to double spend attempt"
	default:
		return "Unknown status: " + status
	}
}
