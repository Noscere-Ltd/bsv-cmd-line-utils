// Package cli provides shared utilities for command-line Bitcoin SV tools.
//
// This package contains common functions used across multiple CLI tools including:
//   - Hex validation with pre-compiled regex for performance
//   - Stdin reading and sanitization
//   - String cleaning utilities
package cli

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// hexRegex is a pre-compiled regex for hex validation.
// Pre-compiling provides ~10-100x performance improvement over regexp.MatchString().
var hexRegex = regexp.MustCompile("^[0-9a-fA-F]+$")

// IsValidHex validates that a string contains only hexadecimal characters (0-9, a-f, A-F).
// Returns true if the string is valid hex, false otherwise.
// Uses pre-compiled regex for optimal performance.
func IsValidHex(s string) bool {
	if s == "" {
		return false
	}

	return hexRegex.MatchString(s)
}

// ReadHexFromReader reads hex data from any io.Reader, cleaning whitespace and control characters.
// It strips all whitespace and control characters, returning only printable ASCII characters.
// This allows for flexible input formatting (newlines, spaces, etc.).
//
// Returns the cleaned hex string and any error encountered during reading.
func ReadHexFromReader(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)

	var result strings.Builder
	// Pre-allocate some capacity for typical hex strings
	result.Grow(256)

	for scanner.Scan() {
		cleaned := CleanString(scanner.Text())
		result.WriteString(cleaned)
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return result.String(), nil
}

// ReadTextFromReader reads plaintext from any io.Reader, preserving spaces and content.
// Only trailing newlines/carriage returns are trimmed. This is suitable for message
// content where whitespace is significant (unlike ReadHexFromReader which strips all whitespace).
func ReadTextFromReader(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(data), "\n\r"), nil
}

// CleanString removes all whitespace and non-printable ASCII characters from a string.
// Only characters with ASCII codes 33-126 (printable, non-space) are retained.
func CleanString(s string) string {
	return strings.Map(func(r rune) rune {
		if r > 32 && r < 127 {
			return r
		}

		return -1
	}, s)
}
