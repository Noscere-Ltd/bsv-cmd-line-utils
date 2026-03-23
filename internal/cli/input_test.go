package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidHex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid hex strings
		{name: "lowercase hex", input: "abcdef", expected: true},
		{name: "uppercase hex", input: "ABCDEF", expected: true},
		{name: "mixed case hex", input: "AbCdEf", expected: true},
		{name: "numeric hex", input: "0123456789", expected: true},
		{name: "full hex charset lowercase", input: "0123456789abcdef", expected: true},
		{name: "full hex charset uppercase", input: "0123456789ABCDEF", expected: true},
		{name: "single character", input: "a", expected: true},
		{name: "long hex string", input: strings.Repeat("deadbeef", 100), expected: true},
		{name: "transaction id length (64 chars)", input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", expected: true},

		// Invalid hex strings
		{name: "empty string", input: "", expected: false},
		{name: "contains g", input: "abcdefg", expected: false},
		{name: "contains space", input: "abc def", expected: false},
		{name: "contains newline", input: "abc\ndef", expected: false},
		{name: "contains tab", input: "abc\tdef", expected: false},
		{name: "leading space", input: " abcdef", expected: false},
		{name: "trailing space", input: "abcdef ", expected: false},
		{name: "special characters", input: "abc!@#", expected: false},
		{name: "unicode characters", input: "abc\u4e16\u754c", expected: false},
		{name: "only spaces", input: "   ", expected: false},
		{name: "mixed valid invalid", input: "abc123xyz", expected: false},
		{name: "hyphen", input: "abc-def", expected: false},
		{name: "underscore", input: "abc_def", expected: false},
		{name: "0x prefix", input: "0xabcdef", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsValidHex(tt.input)
			assert.Equal(t, tt.expected, result, "IsValidHex(%q) = %v, want %v", tt.input, result, tt.expected)
		})
	}
}

func TestReadHexFromReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		// Valid inputs
		{name: "simple hex", input: "abcdef", expected: "abcdef"},
		{name: "hex with newlines", input: "abc\ndef\n123", expected: "abcdef123"},
		{name: "hex with spaces", input: "abc def 123", expected: "abcdef123"},
		{name: "hex with tabs", input: "abc\tdef\t123", expected: "abcdef123"},
		{name: "hex with carriage return", input: "abc\r\ndef", expected: "abcdef"},
		{name: "hex with mixed whitespace", input: "  abc \n def \t 123  ", expected: "abcdef123"},
		{name: "empty input", input: "", expected: ""},
		{name: "only whitespace", input: "   \n\t  ", expected: ""},
		{name: "multiple lines", input: "line1\nline2\nline3", expected: "line1line2line3"},
		{name: "control characters stripped", input: "abc\x00\x01\x02def", expected: "abcdef"},
		{name: "printable ASCII retained", input: "abc!@#$%^&*()def", expected: "abc!@#$%^&*()def"},

		// Boundary cases
		{name: "char at ASCII 33 (exclamation)", input: "a!b", expected: "a!b"},
		{name: "char at ASCII 126 (tilde)", input: "a~b", expected: "a~b"},
		{name: "char at ASCII 32 (space) removed", input: "a b", expected: "ab"},
		{name: "char at ASCII 127 (DEL) removed", input: "a\x7fb", expected: "ab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader := strings.NewReader(tt.input)
			result, err := ReadHexFromReader(reader)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// errorReader is a mock reader that always returns an error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func TestReadHexFromReaderError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("mock read error")
	reader := &errorReader{err: expectedErr}

	_, err := ReadHexFromReader(reader)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestReadHexFromReaderLargeInput(t *testing.T) {
	t.Parallel()

	// Test with a moderately large input (within scanner buffer limits)
	// Default scanner buffer is 64KB, so we use something smaller
	largeHex := strings.Repeat("abcdef123456", 1000)
	reader := strings.NewReader(largeHex)

	result, err := ReadHexFromReader(reader)
	require.NoError(t, err)
	assert.Equal(t, largeHex, result)
}

func TestCleanString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{name: "empty string", input: "", expected: ""},
		{name: "no changes needed", input: "abcdef", expected: "abcdef"},
		{name: "remove spaces", input: "abc def", expected: "abcdef"},
		{name: "remove tabs", input: "abc\tdef", expected: "abcdef"},
		{name: "remove newlines", input: "abc\ndef", expected: "abcdef"},
		{name: "remove carriage returns", input: "abc\rdef", expected: "abcdef"},

		// Control characters
		{name: "remove null bytes", input: "abc\x00def", expected: "abcdef"},
		{name: "remove all control chars", input: "\x00\x01\x02abc\x1f", expected: "abc"},

		// Boundary ASCII values
		{name: "keep ASCII 33 (!)", input: "a!b", expected: "a!b"},
		{name: "keep ASCII 126 (~)", input: "a~b", expected: "a~b"},
		{name: "remove ASCII 32 (space)", input: "a b", expected: "ab"},
		{name: "remove ASCII 127 (DEL)", input: "a\x7fb", expected: "ab"},

		// Printable characters
		{name: "keep special chars", input: "!@#$%^&*()", expected: "!@#$%^&*()"},
		{name: "keep numbers", input: "0123456789", expected: "0123456789"},
		{name: "keep mixed", input: "abc123!@#", expected: "abc123!@#"},

		// Unicode (should be removed as > 127)
		{name: "remove unicode", input: "abc\u4e16\u754cdef", expected: "abcdef"},
		{name: "remove emoji", input: "abc😀def", expected: "abcdef"},

		// Only non-printable
		{name: "only spaces", input: "     ", expected: ""},
		{name: "only control chars", input: "\x00\x01\x02", expected: ""},
		{name: "only newlines", input: "\n\n\n", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := CleanString(tt.input)
			assert.Equal(t, tt.expected, result, "CleanString(%q) = %q, want %q", tt.input, result, tt.expected)
		})
	}
}

// Benchmarks

func BenchmarkIsValidHex(b *testing.B) {
	testCases := []struct {
		name  string
		input string
	}{
		{"short_valid", "abcdef"},
		{"medium_valid", "0123456789abcdef0123456789abcdef"},
		{"long_valid", strings.Repeat("deadbeef", 100)},
		{"txid_length", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		{"invalid", "not-valid-hex"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = IsValidHex(tc.input)
			}
		})
	}
}

func BenchmarkCleanString(b *testing.B) {
	testCases := []struct {
		name  string
		input string
	}{
		{"no_cleaning", "abcdef123456"},
		{"with_spaces", "abc def 123 456"},
		{"with_newlines", "abc\ndef\n123\n456"},
		{"mixed_whitespace", "  abc \n def \t 123  "},
		{"large_input", strings.Repeat("abc def\n", 1000)},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = CleanString(tc.input)
			}
		})
	}
}

func BenchmarkReadHexFromReader(b *testing.B) {
	input := strings.Repeat("abcdef123456\n", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(input)
		_, _ = ReadHexFromReader(reader)
	}
}

// Fuzz tests

func FuzzIsValidHex(f *testing.F) {
	// Add seed corpus
	seeds := []string{
		"",
		"a",
		"abcdef",
		"ABCDEF",
		"0123456789",
		"ghijklmnop",
		"abc def",
		"abc\ndef",
		"!@#$%",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := IsValidHex(input)

		// Verify consistency: if valid, all chars must be hex
		if result {
			for _, c := range input {
				isHexChar := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
				require.True(t, isHexChar, "IsValidHex returned true but found non-hex char: %c", c)
			}
			require.NotEmpty(t, input, "IsValidHex returned true for empty string")
		}
	})
}

func FuzzCleanString(f *testing.F) {
	seeds := []string{
		"",
		"abc",
		"abc def",
		"abc\ndef",
		"\x00\x01\x02",
		"!@#$%^&*()",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := CleanString(input)

		// Verify all characters in result are printable ASCII (33-126)
		for _, c := range result {
			require.True(t, c > 32 && c < 127,
				"CleanString output contains invalid char: %d (%c)", c, c)
		}
	})
}

// Test with bytes.Buffer to ensure interface compatibility
func TestReadHexFromReaderWithBuffer(t *testing.T) {
	t.Parallel()

	input := []byte("abc\ndef\n123")
	buf := bytes.NewBuffer(input)

	result, err := ReadHexFromReader(buf)
	require.NoError(t, err)
	assert.Equal(t, "abcdef123", result)
}

// Test EOF handling
func TestReadHexFromReaderEOF(t *testing.T) {
	t.Parallel()

	// Empty reader should return empty string without error
	reader := strings.NewReader("")
	result, err := ReadHexFromReader(reader)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// Test that reader is fully consumed
func TestReadHexFromReaderFullyConsumes(t *testing.T) {
	t.Parallel()

	input := "abcdef"
	reader := strings.NewReader(input)

	_, err := ReadHexFromReader(reader)
	require.NoError(t, err)

	// Try to read more - should get EOF/empty
	remaining := make([]byte, 10)
	n, err := reader.Read(remaining)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)
}
