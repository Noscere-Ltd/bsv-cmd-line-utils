package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateFee(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feePerKb   uint64
		expected   uint64
	}{
		// Basic calculations
		{
			name:       "single input single output standard fee",
			numInputs:  1,
			numOutputs: 1,
			feePerKb:   1000,
			// (1*148 + 1*34 + 10) * 1000 / 1000 = 192
			expected: 192,
		},
		{
			name:       "two inputs two outputs",
			numInputs:  2,
			numOutputs: 2,
			feePerKb:   1000,
			// (2*148 + 2*34 + 10) * 1000 / 1000 = 374
			expected: 374,
		},
		{
			name:       "large transaction",
			numInputs:  10,
			numOutputs: 5,
			feePerKb:   1000,
			// (10*148 + 5*34 + 10) * 1000 / 1000 = 1660
			expected: 1660,
		},

		// Minimum fee enforcement
		{
			name:       "enforces minimum fee with low fee rate",
			numInputs:  1,
			numOutputs: 1,
			feePerKb:   1, // Very low fee rate
			// Calculated: (192 * 1) / 1000 = 0, but minimum is 100
			expected: minFee,
		},
		{
			name:       "enforces minimum fee with zero fee rate",
			numInputs:  1,
			numOutputs: 1,
			feePerKb:   0,
			expected:   minFee,
		},

		// Edge cases
		{
			name:       "zero inputs zero outputs",
			numInputs:  0,
			numOutputs: 0,
			feePerKb:   1000,
			// (0*148 + 0*34 + 10) * 1000 / 1000 = 10, minimum is 100
			expected: minFee,
		},
		{
			name:       "high fee rate",
			numInputs:  1,
			numOutputs: 1,
			feePerKb:   10000,
			// (192) * 10000 / 1000 = 1920
			expected: 1920,
		},
		{
			name:       "BSV typical fee rate (100 sat/kb)",
			numInputs:  1,
			numOutputs: 2,
			feePerKb:   100,
			// (1*148 + 2*34 + 10) * 100 / 1000 = 22.6 -> 22, minimum is 100
			expected: minFee,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := calculateFee(tt.numInputs, tt.numOutputs, tt.feePerKb)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSelectUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("single UTXO sufficient", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 10000},
		}

		selected, err := selectUTXOs(utxos, 5000, 100)
		require.NoError(t, err)
		require.Len(t, selected, 1)
		assert.Equal(t, "tx1", selected[0].TxHash)
	})

	t.Run("multiple UTXOs needed", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 1000},
			{TxHash: "tx2", TxPos: 0, Value: 2000},
			{TxHash: "tx3", TxPos: 0, Value: 3000},
		}

		// Target 4000 + fee, needs at least 2 UTXOs
		selected, err := selectUTXOs(utxos, 4000, 100)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(selected), 2)

		// Should select largest first (3000, then 2000)
		var totalValue uint64
		for _, u := range selected {
			totalValue += u.Value
		}
		assert.GreaterOrEqual(t, totalValue, uint64(4000))
	})

	t.Run("selects largest first", func(t *testing.T) {
		t.Parallel()

		// UTXOs not in order by value
		utxos := []*UTXO{
			{TxHash: "small", TxPos: 0, Value: 100},
			{TxHash: "large", TxPos: 0, Value: 10000},
			{TxHash: "medium", TxPos: 0, Value: 5000},
		}

		selected, err := selectUTXOs(utxos, 1000, 100)
		require.NoError(t, err)
		require.Len(t, selected, 1)
		assert.Equal(t, "large", selected[0].TxHash)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 1000},
			{TxHash: "tx2", TxPos: 0, Value: 2000},
		}

		// Target much more than available
		_, err := selectUTXOs(utxos, 100000, 100)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient funds")
	})

	t.Run("empty UTXO list", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{}

		_, err := selectUTXOs(utxos, 1000, 100)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no UTXOs available")
	})

	t.Run("exact amount match", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 5100}, // Just enough for 5000 + ~100 fee
		}

		selected, err := selectUTXOs(utxos, 5000, 100)
		require.NoError(t, err)
		require.Len(t, selected, 1)
	})

	t.Run("preserves original slice order", func(t *testing.T) {
		t.Parallel()

		original := []*UTXO{
			{TxHash: "first", TxPos: 0, Value: 100},
			{TxHash: "second", TxPos: 0, Value: 200},
			{TxHash: "third", TxPos: 0, Value: 300},
		}

		// Make a copy to verify original isn't modified
		originalCopy := make([]*UTXO, len(original))
		copy(originalCopy, original)

		_, _ = selectUTXOs(original, 50, 100)

		// Original should be unchanged
		for i, u := range original {
			assert.Equal(t, originalCopy[i].TxHash, u.TxHash)
		}
	})

	t.Run("accounts for increasing fee with more inputs", func(t *testing.T) {
		t.Parallel()

		// Many small UTXOs - fee increases as more are added
		utxos := make([]*UTXO, 20)
		for i := 0; i < 20; i++ {
			utxos[i] = &UTXO{TxHash: "tx", TxPos: uint32(i), Value: 1000}
		}

		// Target that requires multiple UTXOs
		selected, err := selectUTXOs(utxos, 15000, 1000)
		require.NoError(t, err)

		var totalValue uint64
		for _, u := range selected {
			totalValue += u.Value
		}

		// Total should cover target + fee for all selected inputs
		expectedMinFee := calculateFee(len(selected), 2, 1000)
		assert.GreaterOrEqual(t, totalValue, uint64(15000)+expectedMinFee)
	})
}

func TestParseUTXOResponse(t *testing.T) {
	t.Parallel()

	t.Run("valid response with multiple UTXOs", func(t *testing.T) {
		t.Parallel()

		jsonResponse := []byte(`{
			"address": "1ABC...",
			"script": "76a914...",
			"result": [
				{"height": 850000, "tx_pos": 0, "tx_hash": "abc123", "value": 10000, "isSpentInMempoolTx": false, "status": "confirmed"},
				{"height": 850001, "tx_pos": 1, "tx_hash": "def456", "value": 20000, "isSpentInMempoolTx": false, "status": "confirmed"}
			],
			"error": ""
		}`)

		utxos, err := parseUTXOResponse(jsonResponse)
		require.NoError(t, err)
		require.Len(t, utxos, 2)

		assert.Equal(t, "abc123", utxos[0].TxHash)
		assert.Equal(t, uint32(0), utxos[0].TxPos)
		assert.Equal(t, uint64(10000), utxos[0].Value)

		assert.Equal(t, "def456", utxos[1].TxHash)
		assert.Equal(t, uint32(1), utxos[1].TxPos)
		assert.Equal(t, uint64(20000), utxos[1].Value)
	})

	t.Run("filters out spent in mempool", func(t *testing.T) {
		t.Parallel()

		jsonResponse := []byte(`{
			"address": "1ABC...",
			"script": "76a914...",
			"result": [
				{"height": 850000, "tx_pos": 0, "tx_hash": "available", "value": 10000, "isSpentInMempoolTx": false, "status": "confirmed"},
				{"height": 850001, "tx_pos": 1, "tx_hash": "spent", "value": 20000, "isSpentInMempoolTx": true, "status": "confirmed"}
			],
			"error": ""
		}`)

		utxos, err := parseUTXOResponse(jsonResponse)
		require.NoError(t, err)
		require.Len(t, utxos, 1)
		assert.Equal(t, "available", utxos[0].TxHash)
	})

	t.Run("empty result array", func(t *testing.T) {
		t.Parallel()

		jsonResponse := []byte(`{
			"address": "1ABC...",
			"script": "76a914...",
			"result": [],
			"error": ""
		}`)

		utxos, err := parseUTXOResponse(jsonResponse)
		require.NoError(t, err)
		assert.Empty(t, utxos)
	})

	t.Run("API error in response", func(t *testing.T) {
		t.Parallel()

		jsonResponse := []byte(`{
			"address": "",
			"script": "",
			"result": [],
			"error": "Address not found"
		}`)

		_, err := parseUTXOResponse(jsonResponse)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Address not found")
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()

		jsonResponse := []byte(`not valid json`)

		_, err := parseUTXOResponse(jsonResponse)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("all UTXOs spent in mempool", func(t *testing.T) {
		t.Parallel()

		jsonResponse := []byte(`{
			"address": "1ABC...",
			"script": "76a914...",
			"result": [
				{"height": 850000, "tx_pos": 0, "tx_hash": "spent1", "value": 10000, "isSpentInMempoolTx": true, "status": "confirmed"},
				{"height": 850001, "tx_pos": 1, "tx_hash": "spent2", "value": 20000, "isSpentInMempoolTx": true, "status": "confirmed"}
			],
			"error": ""
		}`)

		utxos, err := parseUTXOResponse(jsonResponse)
		require.NoError(t, err)
		assert.Empty(t, utxos)
	})
}

func TestFilterAndDeduplicateUTXOs(t *testing.T) {
	t.Parallel()

	t.Run("no duplicates", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 1000},
			{TxHash: "tx2", TxPos: 0, Value: 2000},
			{TxHash: "tx3", TxPos: 0, Value: 3000},
		}

		result, err := filterAndDeduplicateUTXOs(utxos, "testaddr")
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("removes duplicates by txid:vout", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 1000},
			{TxHash: "tx1", TxPos: 0, Value: 1000}, // Duplicate
			{TxHash: "tx1", TxPos: 1, Value: 2000}, // Different vout, not duplicate
			{TxHash: "tx2", TxPos: 0, Value: 3000},
		}

		result, err := filterAndDeduplicateUTXOs(utxos, "testaddr")
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("preserves order of first occurrence", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "first", TxPos: 0, Value: 1000},
			{TxHash: "second", TxPos: 0, Value: 2000},
			{TxHash: "first", TxPos: 0, Value: 1000}, // Duplicate
		}

		result, err := filterAndDeduplicateUTXOs(utxos, "testaddr")
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "first", result[0].TxHash)
		assert.Equal(t, "second", result[1].TxHash)
	})

	t.Run("empty UTXO list", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{}

		_, err := filterAndDeduplicateUTXOs(utxos, "testaddr")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no UTXOs found")
	})

	t.Run("single UTXO", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 1000},
		}

		result, err := filterAndDeduplicateUTXOs(utxos, "testaddr")
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})

	t.Run("all duplicates become single", func(t *testing.T) {
		t.Parallel()

		utxos := []*UTXO{
			{TxHash: "tx1", TxPos: 0, Value: 1000},
			{TxHash: "tx1", TxPos: 0, Value: 1000},
			{TxHash: "tx1", TxPos: 0, Value: 1000},
		}

		result, err := filterAndDeduplicateUTXOs(utxos, "testaddr")
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

func TestUTXOStruct(t *testing.T) {
	t.Parallel()

	utxo := &UTXO{
		TxHash: "0123456789abcdef",
		TxPos:  2,
		Value:  123456789,
	}

	assert.Equal(t, "0123456789abcdef", utxo.TxHash)
	assert.Equal(t, uint32(2), utxo.TxPos)
	assert.Equal(t, uint64(123456789), utxo.Value)
}

func TestConstants(t *testing.T) {
	t.Parallel()

	// Verify constants have expected values
	assert.Equal(t, 148, inputSize)
	assert.Equal(t, 34, outputSize)
	assert.Equal(t, 10, baseTxSize)
	assert.Equal(t, 100, minFee)
}

// Benchmarks

func BenchmarkCalculateFee(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = calculateFee(5, 3, 1000)
	}
}

func BenchmarkSelectUTXOs(b *testing.B) {
	utxos := make([]*UTXO, 100)
	for i := 0; i < 100; i++ {
		utxos[i] = &UTXO{TxHash: "tx", TxPos: uint32(i), Value: uint64((i + 1) * 1000)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = selectUTXOs(utxos, 50000, 1000)
	}
}

func BenchmarkParseUTXOResponse(b *testing.B) {
	jsonResponse := []byte(`{
		"address": "1ABC...",
		"script": "76a914...",
		"result": [
			{"height": 850000, "tx_pos": 0, "tx_hash": "abc123", "value": 10000, "isSpentInMempoolTx": false, "status": "confirmed"},
			{"height": 850001, "tx_pos": 1, "tx_hash": "def456", "value": 20000, "isSpentInMempoolTx": false, "status": "confirmed"},
			{"height": 850002, "tx_pos": 2, "tx_hash": "ghi789", "value": 30000, "isSpentInMempoolTx": false, "status": "confirmed"}
		],
		"error": ""
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseUTXOResponse(jsonResponse)
	}
}

func BenchmarkFilterAndDeduplicateUTXOs(b *testing.B) {
	utxos := make([]*UTXO, 100)
	for i := 0; i < 100; i++ {
		utxos[i] = &UTXO{TxHash: "tx", TxPos: uint32(i % 50), Value: uint64(i * 1000)} // Some duplicates
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = filterAndDeduplicateUTXOs(utxos, "testaddr")
	}
}
