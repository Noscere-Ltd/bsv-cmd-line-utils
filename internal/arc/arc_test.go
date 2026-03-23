package arc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewARCClient(t *testing.T) {
	t.Parallel()

	t.Run("creates client with URL and API key", func(t *testing.T) {
		t.Parallel()
		client := NewARCClient("https://api.taal.com/arc", "test-api-key")
		require.NotNil(t, client)
		assert.Equal(t, "https://api.taal.com/arc", client.baseURL)
		assert.Equal(t, "test-api-key", client.apiKey)
		require.NotNil(t, client.client)
	})

	t.Run("trims trailing slash from URL", func(t *testing.T) {
		t.Parallel()
		client := NewARCClient("https://api.taal.com/arc/", "key")
		assert.Equal(t, "https://api.taal.com/arc", client.baseURL)
	})

	t.Run("handles empty API key", func(t *testing.T) {
		t.Parallel()
		client := NewARCClient("https://api.taal.com/arc", "")
		assert.Empty(t, client.apiKey)
	})

	t.Run("handles multiple trailing slashes", func(t *testing.T) {
		t.Parallel()
		client := NewARCClient("https://api.taal.com/arc///", "key")
		// TrimSuffix only removes one slash
		assert.Equal(t, "https://api.taal.com/arc//", client.baseURL)
	})
}

func TestBroadcastTransaction(t *testing.T) {
	t.Parallel()

	t.Run("successful broadcast with 201", func(t *testing.T) {
		t.Parallel()

		response := TransactionResponse{
			TxID:      "abc123def456",
			TxStatus:  StatusReceived,
			ExtraInfo: "Transaction received",
			Timestamp: "2024-01-15T10:30:00Z",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/v1/tx", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			// Verify request body
			var req TransactionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
				return
			}
			assert.Equal(t, "0100000001...", req.RawTx)

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "test-key")
		resp, err := client.BroadcastTransaction("0100000001...")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "abc123def456", resp.TxID)
		assert.Equal(t, StatusReceived, resp.TxStatus)
		assert.Equal(t, "Transaction received", resp.ExtraInfo)
	})

	t.Run("successful broadcast with 200", func(t *testing.T) {
		t.Parallel()

		response := TransactionResponse{
			TxID:     "abc123",
			TxStatus: StatusStored,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "test-key")
		resp, err := client.BroadcastTransaction("0100000001...")

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "abc123", resp.TxID)
	})

	t.Run("no authorization header when API key is empty", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(TransactionResponse{TxID: "abc"})
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "")
		_, err := client.BroadcastTransaction("0100000001...")
		require.NoError(t, err)
	})

	t.Run("handles error response with message", func(t *testing.T) {
		t.Parallel()

		errorResp := ErrorResponse{
			Status: 400,
			Code:   106,
			Error:  "Transaction already exists",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResp)
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "test-key")
		resp, err := client.BroadcastTransaction("0100000001...")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Transaction already exists")
		assert.Contains(t, err.Error(), "400")
		assert.Contains(t, err.Error(), "106")
	})

	t.Run("handles error response without message", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("{}"))
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "test-key")
		resp, err := client.BroadcastTransaction("0100000001...")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("handles network error", func(t *testing.T) {
		t.Parallel()

		client := NewARCClient("http://localhost:1", "test-key") // Invalid port
		resp, err := client.BroadcastTransaction("0100000001...")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to send request")
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("not json"))
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "test-key")
		resp, err := client.BroadcastTransaction("0100000001...")

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to decode response")
	})
}

func TestGetTransactionStatus(t *testing.T) {
	t.Parallel()

	t.Run("successful status check", func(t *testing.T) {
		t.Parallel()

		status := TransactionStatus{
			TxID:        "abc123def456",
			TxStatus:    StatusMined,
			ExtraInfo:   "",
			Timestamp:   "2024-01-15T10:30:00Z",
			BlockHash:   "00000000000000000123456789abcdef",
			BlockHeight: 850000,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/v1/tx/abc123def456", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(status)
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "test-key")
		result, err := client.GetTransactionStatus("abc123def456")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "abc123def456", result.TxID)
		assert.Equal(t, StatusMined, result.TxStatus)
		assert.Equal(t, "00000000000000000123456789abcdef", result.BlockHash)
		assert.Equal(t, int64(850000), result.BlockHeight)
	})

	t.Run("handles not found error", func(t *testing.T) {
		t.Parallel()

		errorResp := ErrorResponse{
			Status: 404,
			Code:   100,
			Error:  "Transaction not found",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(errorResp)
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "test-key")
		result, err := client.GetTransactionStatus("nonexistent")

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "Transaction not found")
	})

	t.Run("no authorization header when API key is empty", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(TransactionStatus{TxID: "abc"})
		}))
		defer server.Close()

		client := NewARCClient(server.URL, "")
		_, err := client.GetTransactionStatus("abc")
		require.NoError(t, err)
	})

	t.Run("handles network error", func(t *testing.T) {
		t.Parallel()

		client := NewARCClient("http://localhost:1", "test-key")
		result, err := client.GetTransactionStatus("abc123")

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to send request")
	})
}

func TestIsTransactionFinal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		// Final states
		{name: "MINED is final", status: StatusMined, expected: true},
		{name: "REJECTED is final", status: StatusRejected, expected: true},
		{name: "DOUBLE_SPEND_ATTEMPTED is final", status: StatusDoubleSpend, expected: true},

		// Non-final states
		{name: "RECEIVED is not final", status: StatusReceived, expected: false},
		{name: "STORED is not final", status: StatusStored, expected: false},
		{name: "ANNOUNCED_TO_NETWORK is not final", status: StatusAnnouncedToNetwork, expected: false},
		{name: "SEEN_ON_NETWORK is not final", status: StatusSeenOnNetwork, expected: false},
		{name: "SEEN_BY_NETWORK is not final", status: StatusSeenByNetwork, expected: false},

		// Edge cases
		{name: "empty string is not final", status: "", expected: false},
		{name: "unknown status is not final", status: "UNKNOWN", expected: false},
		{name: "lowercase mined is not final", status: "mined", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsTransactionFinal(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStatusColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{name: "RECEIVED", status: StatusReceived, expected: "#FFD700"},
		{name: "STORED", status: StatusStored, expected: "#FFA500"},
		{name: "ANNOUNCED_TO_NETWORK", status: StatusAnnouncedToNetwork, expected: "#1E90FF"},
		{name: "SEEN_ON_NETWORK", status: StatusSeenOnNetwork, expected: "#32CD32"},
		{name: "SEEN_BY_NETWORK", status: StatusSeenByNetwork, expected: "#32CD32"},
		{name: "MINED", status: StatusMined, expected: "#00FF00"},
		{name: "REJECTED", status: StatusRejected, expected: "#FF0000"},
		{name: "DOUBLE_SPEND_ATTEMPTED", status: StatusDoubleSpend, expected: "#FF4500"},
		{name: "unknown status", status: "UNKNOWN", expected: "#FFFFFF"},
		{name: "empty status", status: "", expected: "#FFFFFF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := GetStatusColor(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStatusDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		contains string
	}{
		{name: "RECEIVED", status: StatusReceived, contains: "received by ARC"},
		{name: "STORED", status: StatusStored, contains: "stored and validated"},
		{name: "ANNOUNCED_TO_NETWORK", status: StatusAnnouncedToNetwork, contains: "announced to the BSV network"},
		{name: "SEEN_ON_NETWORK", status: StatusSeenOnNetwork, contains: "seen on the BSV network"},
		{name: "SEEN_BY_NETWORK", status: StatusSeenByNetwork, contains: "seen on the BSV network"},
		{name: "MINED", status: StatusMined, contains: "mined in a block"},
		{name: "REJECTED", status: StatusRejected, contains: "rejected by the network"},
		{name: "DOUBLE_SPEND_ATTEMPTED", status: StatusDoubleSpend, contains: "double spend"},
		{name: "unknown status", status: "CUSTOM_STATUS", contains: "CUSTOM_STATUS"},
		{name: "empty status", status: "", contains: "Unknown status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := GetStatusDescription(tt.status)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestStatusConstants(t *testing.T) {
	t.Parallel()

	// Verify status constants have expected values
	assert.Equal(t, "RECEIVED", StatusReceived)
	assert.Equal(t, "STORED", StatusStored)
	assert.Equal(t, "ANNOUNCED_TO_NETWORK", StatusAnnouncedToNetwork)
	assert.Equal(t, "SEEN_ON_NETWORK", StatusSeenOnNetwork)
	assert.Equal(t, "SEEN_BY_NETWORK", StatusSeenByNetwork)
	assert.Equal(t, "MINED", StatusMined)
	assert.Equal(t, "REJECTED", StatusRejected)
	assert.Equal(t, "DOUBLE_SPEND_ATTEMPTED", StatusDoubleSpend)
}

func TestTransactionRequestStruct(t *testing.T) {
	t.Parallel()

	req := TransactionRequest{RawTx: "0100000001..."}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded map[string]string
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "0100000001...", decoded["rawTx"])
}

func TestTransactionResponseStruct(t *testing.T) {
	t.Parallel()

	jsonData := `{
		"txid": "abc123",
		"txStatus": "RECEIVED",
		"extraInfo": "some info",
		"timestamp": "2024-01-15T10:30:00Z"
	}`

	var resp TransactionResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	require.NoError(t, err)

	assert.Equal(t, "abc123", resp.TxID)
	assert.Equal(t, "RECEIVED", resp.TxStatus)
	assert.Equal(t, "some info", resp.ExtraInfo)
	assert.Equal(t, "2024-01-15T10:30:00Z", resp.Timestamp)
}

func TestTransactionStatusStruct(t *testing.T) {
	t.Parallel()

	jsonData := `{
		"txid": "abc123",
		"txStatus": "MINED",
		"extraInfo": "",
		"timestamp": "2024-01-15T10:30:00Z",
		"blockHash": "00000000000000000123456789abcdef",
		"blockHeight": 850000
	}`

	var status TransactionStatus
	err := json.Unmarshal([]byte(jsonData), &status)
	require.NoError(t, err)

	assert.Equal(t, "abc123", status.TxID)
	assert.Equal(t, "MINED", status.TxStatus)
	assert.Equal(t, "00000000000000000123456789abcdef", status.BlockHash)
	assert.Equal(t, int64(850000), status.BlockHeight)
}

func TestErrorResponseStruct(t *testing.T) {
	t.Parallel()

	jsonData := `{
		"status": 400,
		"code": 106,
		"error": "Transaction already exists"
	}`

	var errResp ErrorResponse
	err := json.Unmarshal([]byte(jsonData), &errResp)
	require.NoError(t, err)

	assert.Equal(t, 400, errResp.Status)
	assert.Equal(t, 106, errResp.Code)
	assert.Equal(t, "Transaction already exists", errResp.Error)
}
