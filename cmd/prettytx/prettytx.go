// Package main implements a Bitcoin SV transaction parser and visualizer.
//
// This tool parses raw BSV transactions and displays their components in a human-readable,
// colorized format. It breaks down the transaction structure including version, inputs,
// outputs, scripts, and locktime.
//
// Features:
//   - Colorized output for better readability (can be disabled)
//   - Detailed breakdown of all transaction components
//   - Script hex display for inputs and outputs
//   - Address extraction for P2PKH scripts (inputs and outputs)
//   - Satoshi to BSV conversion
//   - Locktime interpretation (block height vs timestamp)
//   - Support for stdin or command-line input
//
// Usage:
//
//	prettytx                                  # Parse from clipboard
//	echo "010000..." | prettytx               # Parse from stdin
//	prettytx -r "010000..."                   # Parse using flag
//	prettytx --no-color                       # Disable colors
//	carve -w <WIF> -a <addr> | prettytx       # Chain with carve
package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/spf13/cobra"
	"golang.design/x/clipboard"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
)

// ANSI color codes for terminal output styling
const (
	colorReset = "\033[0m"  // Reset to default
	colorRed   = "\033[31m" // Red text (errors)
	colorGreen = "\033[32m" // Green text (values, addresses)
	colorWhite = "\033[37m" // White text (headers, structure)
	colorDim   = "\033[2m"  // Dimmed text (labels, annotations)
)

var (
	errNoTransactionProvided = errors.New("no transaction provided")
	errInvalidHexInput       = errors.New("input is not a valid hex string")
)

// newRootCmd builds the cobra command for the prettytx tool.
func newRootCmd() *cobra.Command {
	var (
		raw     string // Raw transaction hex provided via flag
		noColor bool   // Disable colored output
		compact bool   // Enable compact output mode
	)

	cmd := &cobra.Command{
		Use:   "prettytx",
		Short: "Parse and display Bitcoin transaction components",
		Long:  "A command line tool that parses raw Bitcoin transactions and displays their components in human-readable format",
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(raw, noColor, compact)
		},
	}

	cmd.Flags().StringVarP(&raw, "raw", "r", "", "Raw transaction hex to parse")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVarP(&compact, "compact", "c", false, "Enable compact output with truncated scripts")

	return cmd
}

// run handles the main execution flow:
// 1. Reads transaction hex from flag or stdin
// 2. Validates the hex string
// 3. Parses and displays the transaction
func run(raw string, noColor, compact bool) error {
	txString, err := getTransactionHex(raw)
	if err != nil {
		return err
	}

	if txString == "" {
		return errNoTransactionProvided
	}

	// Check the string to ensure it is a hex string
	if !cli.IsValidHex(txString) {
		return errInvalidHexInput
	}

	// Parse and display transaction
	return parseTransaction(txString, noColor, compact)
}

// getTransactionHex reads transaction hex from flag, stdin, or clipboard.
// Priority: 1) --raw flag, 2) stdin (if piped), 3) clipboard
func getTransactionHex(raw string) (string, error) {
	// Check flag first
	if raw != "" {
		return raw, nil
	}

	// Check if stdin has data (is piped)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// stdin has data piped to it
		return cli.ReadHexFromReader(os.Stdin)
	}

	// No flag or stdin, try clipboard
	return readFromClipboard()
}

// readFromClipboard reads a hex string from the system clipboard.
func readFromClipboard() (string, error) {
	if err := clipboard.Init(); err != nil {
		return "", fmt.Errorf("clipboard not available: %w", err)
	}

	data := clipboard.Read(clipboard.FmtText)
	if len(data) == 0 {
		return "", nil
	}

	// Clean up the clipboard content (trim whitespace)
	content := strings.TrimSpace(string(data))

	return content, nil
}

// printer renders transaction output, honoring color and compact-mode settings.
type printer struct {
	noColor bool
	compact bool
}

// c applies ANSI color codes to text if color output is enabled.
// Returns plain text if --no-color flag is set, otherwise returns colorized text.
func (p printer) c(color, text string) string {
	if p.noColor {
		return text
	}

	return color + text + colorReset
}

// truncateHex truncates a hex string if compact mode is enabled and it exceeds maxLen.
func (p printer) truncateHex(hexStr string, maxLen int) string {
	if !p.compact || len(hexStr) <= maxLen {
		return hexStr
	}

	return hexStr[:maxLen] + "..."
}

// printf writes a colorized line to stdout.
func (p printer) printf(format string, args ...any) error {
	_, err := fmt.Fprintf(os.Stdout, format, args...)
	return err
}

// println writes a colorized line to stdout.
func (p printer) println(args ...any) error {
	_, err := fmt.Fprintln(os.Stdout, args...)
	return err
}

// parseTransaction decodes and displays a raw Bitcoin transaction in human-readable format.
func parseTransaction(rawTx string, noColor, compact bool) error {
	// Decode hex to bytes
	txBytes, err := hex.DecodeString(rawTx)
	if err != nil {
		return fmt.Errorf("decoding hex: %w", err)
	}

	// Parse transaction using BSV SDK
	tx, err := transaction.NewTransactionFromBytes(txBytes)
	if err != nil {
		return fmt.Errorf("parsing transaction: %w", err)
	}

	// Display transaction breakdown
	p := printer{noColor: noColor, compact: compact}
	txid := tx.TxID().String()

	for _, step := range []func() error{
		func() error { return p.printHeader(txid) },
		func() error { return p.printVersion(tx) },
		func() error { return p.printInputs(tx) },
		func() error { return p.printOutputs(tx) },
		func() error { return p.printLocktime(tx) },
		func() error { return p.printFooter(tx) },
	} {
		if err := step(); err != nil {
			return err
		}
	}

	return nil
}

// printHeader prints the transaction breakdown header.
func (p printer) printHeader(txid string) error {
	if err := p.printf("%s %s\n", p.c(colorDim, "TX ID:"), p.c(colorGreen, txid)); err != nil {
		return err
	}

	return p.println(p.c(colorWhite, "────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────"))
}

// printVersion prints the transaction version.
func (p printer) printVersion(tx *transaction.Transaction) error {
	return p.printf("%s %d %s\n",
		p.c(colorDim, "Version:"),
		tx.Version,
		p.c(colorDim, fmt.Sprintf("(0x%08x)", tx.Version)))
}

// printInputs prints the transaction inputs section.
func (p printer) printInputs(tx *transaction.Transaction) error {
	inputCount := len(tx.Inputs)
	if err := p.printf("%s %d\n", p.c(colorDim, "Inputs:"), inputCount); err != nil {
		return err
	}

	for i, input := range tx.Inputs {
		if err := p.printInput(i, input); err != nil {
			return err
		}
	}

	return nil
}

// printInput prints a single transaction input.
func (p printer) printInput(index int, input *transaction.TransactionInput) error {
	if err := p.printf("\n%s\n", p.c(colorWhite, fmt.Sprintf("INPUT #%d", index))); err != nil {
		return err
	}

	// Previous transaction ID and output index on same line
	if input.SourceTXID != nil {
		if err := p.printf("  %s %s:%d\n",
			p.c(colorDim, "Prev:"),
			p.c(colorGreen, input.SourceTXID.String()),
			input.SourceTxOutIndex); err != nil {
			return err
		}
	} else {
		if err := p.printf("  %s %s\n", p.c(colorDim, "Prev:"), p.c(colorRed, "(null)")); err != nil {
			return err
		}
	}

	// Script
	if err := p.printUnlockingScript(input.UnlockingScript); err != nil {
		return err
	}

	// Sequence number
	return p.printf("  %s %d %s\n",
		p.c(colorDim, "Sequence:"),
		input.SequenceNumber,
		p.c(colorDim, fmt.Sprintf("(0x%08x)", input.SequenceNumber)))
}

// printUnlockingScript prints the unlocking script details for an input.
func (p printer) printUnlockingScript(unlockingScript *script.Script) error {
	if unlockingScript == nil {
		return p.printf("  %s %s\n", p.c(colorDim, "Script:"), p.c(colorDim, "(empty)"))
	}

	scriptBytes := *unlockingScript
	scriptHex := scriptBytes.String()
	scriptLen := len(scriptBytes)

	if err := p.printf("  %s %s %s\n",
		p.c(colorDim, "Script:"),
		p.c(colorDim, p.truncateHex(scriptHex, 64)),
		p.c(colorDim, fmt.Sprintf("(%d bytes)", scriptLen))); err != nil {
		return err
	}

	// Try to extract address from P2PKH unlocking script
	addr := extractAddressFromUnlockingScript(unlockingScript)
	if addr == "" {
		return nil
	}

	return p.printf("  %s %s\n", p.c(colorDim, "Address:"), p.c(colorGreen, addr))
}

// printOutputs prints the transaction outputs section.
func (p printer) printOutputs(tx *transaction.Transaction) error {
	outputCount := len(tx.Outputs)
	if err := p.printf("%s %d\n", p.c(colorDim, "Outputs:"), outputCount); err != nil {
		return err
	}

	for i, output := range tx.Outputs {
		if err := p.printOutput(i, output); err != nil {
			return err
		}
	}

	return nil
}

// printOutput prints a single transaction output.
func (p printer) printOutput(index int, output *transaction.TransactionOutput) error {
	if err := p.printf("\n%s\n", p.c(colorWhite, fmt.Sprintf("OUTPUT #%d", index))); err != nil {
		return err
	}

	// Value in satoshis
	satoshis := output.Satoshis

	btc := float64(satoshis) / 100000000.0
	if err := p.printf("  %s %s %s\n",
		p.c(colorDim, "Value:"),
		p.c(colorGreen, fmt.Sprintf("%d sats", satoshis)),
		p.c(colorDim, fmt.Sprintf("(%.8f BSV)", btc))); err != nil {
		return err
	}

	// Locking script
	return p.printLockingScript(output.LockingScript)
}

// printLockingScript prints the locking script details for an output.
func (p printer) printLockingScript(lockingScript *script.Script) error {
	if lockingScript == nil {
		return p.printf("  %s %s\n", p.c(colorDim, "Script:"), p.c(colorDim, "(empty)"))
	}

	scriptBytes := *lockingScript
	scriptHex := scriptBytes.String()
	scriptLen := len(scriptBytes)

	if err := p.printf("  %s %s %s\n",
		p.c(colorDim, "Script:"),
		p.c(colorDim, p.truncateHex(scriptHex, 64)),
		p.c(colorDim, fmt.Sprintf("(%d bytes)", scriptLen))); err != nil {
		return err
	}

	// Try to extract P2PKH address
	addr := extractP2PKHAddress(lockingScript, true)
	if addr == "" {
		return nil
	}

	return p.printf("  %s %s\n", p.c(colorDim, "Address:"), p.c(colorGreen, addr))
}

// printLocktime prints the transaction locktime.
func (p printer) printLocktime(tx *transaction.Transaction) error {
	var lockInfo string

	switch {
	case tx.LockTime == 0:
		lockInfo = "(Not locked)"
	case tx.LockTime < 500000000:
		lockInfo = fmt.Sprintf("(Block %d)", tx.LockTime)
	default:
		lockInfo = fmt.Sprintf("(Timestamp %d)", tx.LockTime)
	}

	return p.printf("\n%s %d %s\n",
		p.c(colorDim, "nLockTime:"),
		tx.LockTime,
		p.c(colorDim, lockInfo))
}

// printFooter prints the transaction footer with TXID.
func (p printer) printFooter(tx *transaction.Transaction) error {
	if err := p.println(p.c(colorWhite, "────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────")); err != nil {
		return err
	}

	return p.printf("%s %s\n", p.c(colorDim, "TX ID:"), p.c(colorGreen, tx.TxID().String()))
}

// isP2PKH checks if a locking script is a standard P2PKH (Pay-to-PubKey-Hash) script.
// P2PKH scripts have the pattern: OP_DUP OP_HASH160 <20-byte-hash> OP_EQUALVERIFY OP_CHECKSIG
// This is exactly 25 bytes: 76 a9 14 <20 bytes> 88 ac
func isP2PKH(scriptBytes *script.Script) bool {
	if scriptBytes == nil {
		return false
	}

	bytes := []byte(*scriptBytes)

	// Check length (must be exactly 25 bytes)
	if len(bytes) != 25 {
		return false
	}

	// Check the P2PKH pattern
	return bytes[0] == 0x76 && // OP_DUP
		bytes[1] == 0xa9 && // OP_HASH160
		bytes[2] == 0x14 && // Push 20 bytes
		bytes[23] == 0x88 && // OP_EQUALVERIFY
		bytes[24] == 0xac // OP_CHECKSIG
}

// extractP2PKHAddress extracts the address from a P2PKH locking script.
// Returns the address string if successful, empty string otherwise.
func extractP2PKHAddress(scriptBytes *script.Script, mainnet bool) string {
	if !isP2PKH(scriptBytes) {
		return ""
	}

	bytes := []byte(*scriptBytes)

	// Extract the 20-byte public key hash (bytes 3-22)
	pubKeyHash := bytes[3:23]

	// Create an address from the hash
	addr, err := script.NewAddressFromPublicKeyHash(pubKeyHash, mainnet)
	if err != nil {
		return ""
	}

	return addr.AddressString
}

// extractAddressFromUnlockingScript attempts to extract a mainnet address from a P2PKH unlocking script.
// P2PKH unlocking scripts contain: <signature> <pubKey>
// This function extracts the public key and derives the address from it.
// Returns the address string if successful, empty string otherwise.
func extractAddressFromUnlockingScript(scriptBytes *script.Script) string {
	if scriptBytes == nil {
		return ""
	}

	bytes := []byte(*scriptBytes)
	if len(bytes) == 0 {
		return ""
	}

	// Parse the script to extract the public key
	pubKeyBytes := extractPublicKeyFromScript(bytes)
	if len(pubKeyBytes) == 0 {
		return ""
	}

	// Try to parse the public key
	pubKey, err := ec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return ""
	}

	// Derive the address from the public key
	addr, err := script.NewAddressFromPublicKey(pubKey, true)
	if err != nil {
		return ""
	}

	return addr.AddressString
}

// readPushData reads a single opcode at i. If it is a push-data opcode
// (direct push or OP_PUSHDATA1), it returns the pushed bytes and the index
// following the push. stop reports whether the script is malformed/truncated
// and parsing should stop; a non-push opcode returns stop=false with nil data
// so the caller just advances past it.
func readPushData(bytes []byte, i int) (data []byte, next int, stop bool) {
	if i >= len(bytes) {
		return nil, i, true
	}

	opcode := bytes[i]
	i++

	var length int

	switch {
	case opcode > 0 && opcode <= 75:
		// Direct push of N bytes
		length = int(opcode)
	case opcode == 0x4c: // OP_PUSHDATA1
		if i >= len(bytes) {
			return nil, i, true
		}

		length = int(bytes[i])
		i++
	default:
		return nil, i, false
	}

	if i+length > len(bytes) {
		return nil, i, true
	}

	return bytes[i : i+length], i + length, false
}

// extractPublicKeyFromScript parses a script to extract the public key.
// In a typical P2PKH unlocking script:
// - First comes the signature (variable length, typically ~72 bytes)
// - Then comes the public key (33 or 65 bytes)
func extractPublicKeyFromScript(bytes []byte) []byte {
	var pubKeyBytes []byte

	for i := 0; i < len(bytes); {
		data, next, stop := readPushData(bytes, i)
		if stop {
			break
		}

		i = next

		// Check if this looks like a public key (33 or 65 bytes)
		if len(data) == 33 || len(data) == 65 {
			pubKeyBytes = data
		}
	}

	return pubKeyBytes
}

// main is the entry point for the prettytx command.
// It executes the cobra root command which handles flag parsing and command execution.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}
