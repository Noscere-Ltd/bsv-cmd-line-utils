package main

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsP2PKH(t *testing.T) {
	t.Parallel()

	t.Run("valid P2PKH script", func(t *testing.T) {
		t.Parallel()

		// Standard P2PKH: OP_DUP OP_HASH160 <20 bytes> OP_EQUALVERIFY OP_CHECKSIG
		// 76 a9 14 [20 bytes pubkey hash] 88 ac
		scriptBytes := []byte{
			0x76, 0xa9, 0x14, // OP_DUP, OP_HASH160, PUSH 20 bytes
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
			0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
			0x10, 0x11, 0x12, 0x13, // 20 byte pubkey hash
			0x88, 0xac, // OP_EQUALVERIFY, OP_CHECKSIG
		}
		s := script.Script(scriptBytes)

		assert.True(t, isP2PKH(&s))
	})

	t.Run("nil script", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isP2PKH(nil))
	})

	t.Run("empty script", func(t *testing.T) {
		t.Parallel()
		s := script.Script([]byte{})
		assert.False(t, isP2PKH(&s))
	})

	t.Run("too short", func(t *testing.T) {
		t.Parallel()
		s := script.Script([]byte{0x76, 0xa9, 0x14})
		assert.False(t, isP2PKH(&s))
	})

	t.Run("too long", func(t *testing.T) {
		t.Parallel()
		scriptBytes := make([]byte, 26)
		scriptBytes[0] = 0x76
		scriptBytes[1] = 0xa9
		scriptBytes[2] = 0x14
		scriptBytes[23] = 0x88
		scriptBytes[24] = 0xac
		s := script.Script(scriptBytes)
		assert.False(t, isP2PKH(&s))
	})

	t.Run("wrong first opcode", func(t *testing.T) {
		t.Parallel()
		scriptBytes := make([]byte, 25)
		scriptBytes[0] = 0x00 // Wrong - should be 0x76 (OP_DUP)
		scriptBytes[1] = 0xa9
		scriptBytes[2] = 0x14
		scriptBytes[23] = 0x88
		scriptBytes[24] = 0xac
		s := script.Script(scriptBytes)
		assert.False(t, isP2PKH(&s))
	})

	t.Run("wrong second opcode", func(t *testing.T) {
		t.Parallel()
		scriptBytes := make([]byte, 25)
		scriptBytes[0] = 0x76
		scriptBytes[1] = 0x00 // Wrong - should be 0xa9 (OP_HASH160)
		scriptBytes[2] = 0x14
		scriptBytes[23] = 0x88
		scriptBytes[24] = 0xac
		s := script.Script(scriptBytes)
		assert.False(t, isP2PKH(&s))
	})

	t.Run("wrong push length", func(t *testing.T) {
		t.Parallel()
		scriptBytes := make([]byte, 25)
		scriptBytes[0] = 0x76
		scriptBytes[1] = 0xa9
		scriptBytes[2] = 0x15 // Wrong - should be 0x14 (push 20 bytes)
		scriptBytes[23] = 0x88
		scriptBytes[24] = 0xac
		s := script.Script(scriptBytes)
		assert.False(t, isP2PKH(&s))
	})

	t.Run("wrong equalverify opcode", func(t *testing.T) {
		t.Parallel()
		scriptBytes := make([]byte, 25)
		scriptBytes[0] = 0x76
		scriptBytes[1] = 0xa9
		scriptBytes[2] = 0x14
		scriptBytes[23] = 0x00 // Wrong - should be 0x88 (OP_EQUALVERIFY)
		scriptBytes[24] = 0xac
		s := script.Script(scriptBytes)
		assert.False(t, isP2PKH(&s))
	})

	t.Run("wrong checksig opcode", func(t *testing.T) {
		t.Parallel()
		scriptBytes := make([]byte, 25)
		scriptBytes[0] = 0x76
		scriptBytes[1] = 0xa9
		scriptBytes[2] = 0x14
		scriptBytes[23] = 0x88
		scriptBytes[24] = 0x00 // Wrong - should be 0xac (OP_CHECKSIG)
		s := script.Script(scriptBytes)
		assert.False(t, isP2PKH(&s))
	})

	t.Run("exactly 24 bytes (one too short)", func(t *testing.T) {
		t.Parallel()
		scriptBytes := make([]byte, 24)
		scriptBytes[0] = 0x76
		scriptBytes[1] = 0xa9
		scriptBytes[2] = 0x14
		s := script.Script(scriptBytes)
		assert.False(t, isP2PKH(&s))
	})
}

func TestExtractP2PKHAddress(t *testing.T) {
	t.Parallel()

	t.Run("valid P2PKH mainnet", func(t *testing.T) {
		t.Parallel()

		// Create a valid P2PKH script with a known pubkey hash
		// Using a recognizable pattern
		pubKeyHash := []byte{
			0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67,
			0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67,
			0x89, 0xab, 0xcd, 0xef,
		}

		scriptBytes := append([]byte{0x76, 0xa9, 0x14}, pubKeyHash...)
		scriptBytes = append(scriptBytes, 0x88, 0xac)

		s := script.Script(scriptBytes)
		addr := extractP2PKHAddress(&s, true) // mainnet

		// Should return a valid address string starting with '1' for mainnet
		assert.NotEmpty(t, addr)
		if addr != "" {
			assert.True(t, addr[0] == '1' || addr[0] == '3', "Mainnet address should start with 1 or 3")
		}
	})

	t.Run("valid P2PKH testnet", func(t *testing.T) {
		t.Parallel()

		pubKeyHash := []byte{
			0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67,
			0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67,
			0x89, 0xab, 0xcd, 0xef,
		}

		scriptBytes := append([]byte{0x76, 0xa9, 0x14}, pubKeyHash...)
		scriptBytes = append(scriptBytes, 0x88, 0xac)

		s := script.Script(scriptBytes)
		addr := extractP2PKHAddress(&s, false) // testnet

		// Should return a valid address string starting with 'm' or 'n' for testnet
		assert.NotEmpty(t, addr)
		if addr != "" {
			assert.True(t, addr[0] == 'm' || addr[0] == 'n', "Testnet address should start with m or n")
		}
	})

	t.Run("non-P2PKH script", func(t *testing.T) {
		t.Parallel()

		// Not a P2PKH script (too short)
		scriptBytes := []byte{0x76, 0xa9}
		s := script.Script(scriptBytes)

		addr := extractP2PKHAddress(&s, true)
		assert.Empty(t, addr)
	})

	t.Run("nil script", func(t *testing.T) {
		t.Parallel()
		addr := extractP2PKHAddress(nil, true)
		assert.Empty(t, addr)
	})
}

func TestExtractPublicKeyFromScript(t *testing.T) {
	t.Parallel()

	t.Run("typical P2PKH unlocking script with compressed pubkey", func(t *testing.T) {
		t.Parallel()

		// Simulated P2PKH unlocking script: <sig 72 bytes> <pubkey 33 bytes>
		sig := make([]byte, 72)
		pubKey := make([]byte, 33)
		pubKey[0] = 0x02 // Compressed pubkey prefix

		// Script: <push 72> <sig> <push 33> <pubkey>
		scriptBytes := append([]byte{72}, sig...)
		scriptBytes = append(scriptBytes, 33)
		scriptBytes = append(scriptBytes, pubKey...)

		result := extractPublicKeyFromScript(scriptBytes)
		require.Len(t, result, 33)
		assert.Equal(t, byte(0x02), result[0])
	})

	t.Run("uncompressed pubkey (65 bytes)", func(t *testing.T) {
		t.Parallel()

		sig := make([]byte, 72)
		pubKey := make([]byte, 65)
		pubKey[0] = 0x04 // Uncompressed pubkey prefix

		scriptBytes := append([]byte{72}, sig...)
		scriptBytes = append(scriptBytes, 65)
		scriptBytes = append(scriptBytes, pubKey...)

		result := extractPublicKeyFromScript(scriptBytes)
		require.Len(t, result, 65)
		assert.Equal(t, byte(0x04), result[0])
	})

	t.Run("empty script", func(t *testing.T) {
		t.Parallel()

		result := extractPublicKeyFromScript([]byte{})
		assert.Empty(t, result)
	})

	t.Run("script with only signature (no pubkey)", func(t *testing.T) {
		t.Parallel()

		// Only a signature, no pubkey
		sig := make([]byte, 72)
		scriptBytes := append([]byte{72}, sig...)

		result := extractPublicKeyFromScript(scriptBytes)
		assert.Empty(t, result) // 72 bytes is not a valid pubkey length
	})

	t.Run("script with non-standard length data", func(t *testing.T) {
		t.Parallel()

		// Data that's not 33 or 65 bytes
		data := make([]byte, 50)
		scriptBytes := append([]byte{50}, data...)

		result := extractPublicKeyFromScript(scriptBytes)
		assert.Empty(t, result)
	})

	t.Run("truncated script", func(t *testing.T) {
		t.Parallel()

		// Push 72 bytes but only provide 50
		scriptBytes := append([]byte{72}, make([]byte, 50)...)

		result := extractPublicKeyFromScript(scriptBytes)
		// Should handle gracefully, not panic
		assert.Empty(t, result)
	})

	t.Run("OP_PUSHDATA1 with compressed pubkey", func(t *testing.T) {
		t.Parallel()

		sig := make([]byte, 72)
		pubKey := make([]byte, 33)
		pubKey[0] = 0x03 // Compressed pubkey prefix (odd y)

		// Using OP_PUSHDATA1 (0x4c) for pubkey
		scriptBytes := append([]byte{72}, sig...)
		scriptBytes = append(scriptBytes, 0x4c, 33) // OP_PUSHDATA1, length 33
		scriptBytes = append(scriptBytes, pubKey...)

		result := extractPublicKeyFromScript(scriptBytes)
		require.Len(t, result, 33)
		assert.Equal(t, byte(0x03), result[0])
	})

	t.Run("multiple push operations - extracts last valid pubkey", func(t *testing.T) {
		t.Parallel()

		data1 := make([]byte, 20)
		pubKey1 := make([]byte, 33)
		pubKey1[0] = 0x02
		pubKey2 := make([]byte, 33)
		pubKey2[0] = 0x03 // Different prefix

		// <push 20> <data> <push 33> <pubkey1> <push 33> <pubkey2>
		scriptBytes := append([]byte{20}, data1...)
		scriptBytes = append(scriptBytes, 33)
		scriptBytes = append(scriptBytes, pubKey1...)
		scriptBytes = append(scriptBytes, 33)
		scriptBytes = append(scriptBytes, pubKey2...)

		result := extractPublicKeyFromScript(scriptBytes)
		require.Len(t, result, 33)
		// Should get the last valid pubkey
		assert.Equal(t, byte(0x03), result[0])
	})

	t.Run("OP_PUSHDATA1 truncated length byte", func(t *testing.T) {
		t.Parallel()

		// OP_PUSHDATA1 but no length byte following
		scriptBytes := []byte{0x4c}

		result := extractPublicKeyFromScript(scriptBytes)
		assert.Empty(t, result)
	})

	t.Run("OP_PUSHDATA1 truncated data", func(t *testing.T) {
		t.Parallel()

		// OP_PUSHDATA1, length 33, but only 10 bytes of data
		scriptBytes := make([]byte, 0, 12)
		scriptBytes = append(scriptBytes, 0x4c, 33)
		scriptBytes = append(scriptBytes, make([]byte, 10)...)

		result := extractPublicKeyFromScript(scriptBytes)
		assert.Empty(t, result)
	})
}

func TestC(t *testing.T) {
	// This test manipulates the global noColor variable
	// and should not run in parallel with other tests that use it

	t.Run("color enabled", func(t *testing.T) {
		originalNoColor := noColor
		noColor = false
		defer func() { noColor = originalNoColor }()

		result := c(colorRed, "test")
		assert.Contains(t, result, colorRed)
		assert.Contains(t, result, "test")
		assert.Contains(t, result, colorReset)
	})

	t.Run("color disabled", func(t *testing.T) {
		originalNoColor := noColor
		noColor = true
		defer func() { noColor = originalNoColor }()

		result := c(colorRed, "test")
		assert.Equal(t, "test", result)
		assert.NotContains(t, result, colorRed)
		assert.NotContains(t, result, colorReset)
	})

	t.Run("empty text", func(t *testing.T) {
		originalNoColor := noColor
		noColor = false
		defer func() { noColor = originalNoColor }()

		result := c(colorGreen, "")
		assert.Equal(t, colorGreen+colorReset, result)
	})
}

func TestColorConstants(t *testing.T) {
	t.Parallel()

	// Verify ANSI color codes are correct
	assert.Equal(t, "\033[0m", colorReset)
	assert.Equal(t, "\033[31m", colorRed)
	assert.Equal(t, "\033[32m", colorGreen)
	assert.Equal(t, "\033[37m", colorWhite)
	assert.Equal(t, "\033[2m", colorDim)
}

// Benchmark tests

func BenchmarkIsP2PKH(b *testing.B) {
	scriptBytes := []byte{
		0x76, 0xa9, 0x14,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13,
		0x88, 0xac,
	}
	s := script.Script(scriptBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isP2PKH(&s)
	}
}

func BenchmarkExtractPublicKeyFromScript(b *testing.B) {
	sig := make([]byte, 72)
	pubKey := make([]byte, 33)
	pubKey[0] = 0x02

	scriptBytes := append([]byte{72}, sig...)
	scriptBytes = append(scriptBytes, 33)
	scriptBytes = append(scriptBytes, pubKey...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractPublicKeyFromScript(scriptBytes)
	}
}

// Test with real-world-like data

func TestIsP2PKHWithRealPattern(t *testing.T) {
	t.Parallel()

	// Real P2PKH locking script pattern for a known address
	// This is the script pattern, not actual address data
	validP2PKH := []byte{
		0x76,                                           // OP_DUP
		0xa9,                                           // OP_HASH160
		0x14,                                           // Push 20 bytes
		0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, // 20 byte hash (example)
		0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0,
		0x12, 0x34, 0x56, 0x78,
		0x88, // OP_EQUALVERIFY
		0xac, // OP_CHECKSIG
	}

	s := script.Script(validP2PKH)
	assert.True(t, isP2PKH(&s))
}

func TestExtractAddressFromUnlockingScript(t *testing.T) {
	t.Parallel()

	t.Run("nil script", func(t *testing.T) {
		t.Parallel()
		addr := extractAddressFromUnlockingScript(nil, true)
		assert.Empty(t, addr)
	})

	t.Run("empty script", func(t *testing.T) {
		t.Parallel()
		s := script.Script([]byte{})
		addr := extractAddressFromUnlockingScript(&s, true)
		assert.Empty(t, addr)
	})

	t.Run("script without valid pubkey", func(t *testing.T) {
		t.Parallel()

		// Just a signature, no pubkey
		sig := make([]byte, 72)
		scriptBytes := append([]byte{72}, sig...)
		s := script.Script(scriptBytes)

		addr := extractAddressFromUnlockingScript(&s, true)
		assert.Empty(t, addr)
	})

	t.Run("script with invalid pubkey bytes", func(t *testing.T) {
		t.Parallel()

		// 33 bytes but not a valid pubkey format
		sig := make([]byte, 72)
		invalidPubKey := make([]byte, 33)
		invalidPubKey[0] = 0xFF // Invalid prefix

		scriptBytes := append([]byte{72}, sig...)
		scriptBytes = append(scriptBytes, 33)
		scriptBytes = append(scriptBytes, invalidPubKey...)
		s := script.Script(scriptBytes)

		addr := extractAddressFromUnlockingScript(&s, true)
		// May or may not return empty depending on SDK behavior
		// The important thing is it doesn't panic
		_ = addr
	})
}
