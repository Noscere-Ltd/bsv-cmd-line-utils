// Package main implements a Bitcoin SV transaction element picker for pipeline processing.
//
// This tool extracts specific parts from raw BSV transactions and outputs them as hex strings
// for integration with other tools in a Unix-style pipeline.
//
// Features:
//   - Extract complete serialized inputs or outputs
//   - Extract individual fields (scripts, values, prevtxid, sequence, etc.)
//   - Extract transaction-level fields (version, locktime, txid)
//   - Support for multiple selections in one call
//   - Flexible input: argument, flag, or stdin
//
// Usage:
//
//	pick <rawtx> --output 0                     # Get first output (serialized)
//	pick <rawtx> --output-script 0              # Get first output's locking script
//	pick <rawtx> --input 0 --input 1            # Get first two inputs
//	pick <rawtx> --version --locktime           # Get version and locktime
//	echo <rawtx> | pick --txid                  # Get transaction ID from stdin
//	getraw <txid> | pick --output 0             # Chain with getraw
package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
	"github.com/spf13/cobra"
)

// Command-line flags
var (
	raw string // Raw transaction hex provided via flag

	// Output selectors (can be used multiple times)
	outputs       []int // Complete serialized outputs
	outputScripts []int // Output locking scripts only
	outputValues  []int // Output values only

	// Input selectors (can be used multiple times)
	inputs         []int // Complete serialized inputs
	inputScripts   []int // Input unlocking scripts only
	inputPrevTxIDs []int // Input previous txids only
	inputPrevOuts  []int // Input previous output indices only
	inputSequences []int // Input sequence numbers only

	// Transaction-level selectors
	getVersion  bool // Get version field
	getLocktime bool // Get locktime field
	getTxID     bool // Get transaction ID
)

// rootCmd is the main cobra command for the pick tool.
var rootCmd = &cobra.Command{
	Use:   "pick [rawtx]",
	Short: "Extract parts from a Bitcoin transaction",
	Long: `A command line tool that extracts specific parts from raw Bitcoin transactions
and outputs them as hex strings for pipeline processing.

Supports selecting outputs, inputs, and transaction-level fields.
Multiple selections can be combined in one call.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, args)
	},
}

// run handles the main execution flow.
func run(cmd *cobra.Command, args []string) error {
	// Check if any selector was specified
	if !hasAnySelector() {
		cmd.Help()
		return fmt.Errorf("no selector specified")
	}

	// Get transaction hex
	txHex, err := getTransactionHex(args)
	if err != nil {
		return err
	}

	if txHex == "" {
		cmd.Help()
		return fmt.Errorf("no transaction provided")
	}

	// Validate hex
	if !cli.IsValidHex(txHex) {
		return fmt.Errorf("input is not a valid hex string")
	}

	// Parse the transaction
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return fmt.Errorf("decoding hex: %w", err)
	}

	tx, err := transaction.NewTransactionFromBytes(txBytes)
	if err != nil {
		return fmt.Errorf("parsing transaction: %w", err)
	}

	// Extract and output selected elements
	return extractAndOutput(tx)
}

// hasAnySelector checks if any selection flag was provided.
func hasAnySelector() bool {
	return len(outputs) > 0 ||
		len(outputScripts) > 0 ||
		len(outputValues) > 0 ||
		len(inputs) > 0 ||
		len(inputScripts) > 0 ||
		len(inputPrevTxIDs) > 0 ||
		len(inputPrevOuts) > 0 ||
		len(inputSequences) > 0 ||
		getVersion ||
		getLocktime ||
		getTxID
}

// getTransactionHex reads transaction hex from argument, flag, stdin, or file URL.
func getTransactionHex(args []string) (string, error) {
	// Check argument first
	if len(args) > 0 {
		return resolveInput(args[0])
	}

	// Check flag
	if raw != "" {
		return resolveInput(raw)
	}

	// Check stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

// resolveInput handles hex string or file:// URL input.
func resolveInput(input string) (string, error) {
	// Check if it's a file URL
	if path, found := strings.CutPrefix(input, "file://"); found {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading file: %w", err)
		}
		return cli.CleanString(string(data)), nil
	}

	// Check if it's an HTTP/HTTPS URL
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		resp, err := http.Get(input)
		if err != nil {
			return "", fmt.Errorf("fetching URL: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("reading response: %w", err)
		}
		return cli.CleanString(string(data)), nil
	}

	// It's a raw hex string
	return input, nil
}

// extractAndOutput extracts selected elements and prints them to stdout.
func extractAndOutput(tx *transaction.Transaction) error {
	// Transaction-level fields
	if getVersion {
		fmt.Println(encodeUint32LE(tx.Version))
	}

	if getTxID {
		fmt.Println(tx.TxID().String())
	}

	// Output selections
	for _, idx := range outputs {
		hex, err := getSerializedOutput(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	for _, idx := range outputScripts {
		hex, err := getOutputScript(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	for _, idx := range outputValues {
		hex, err := getOutputValue(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	// Input selections
	for _, idx := range inputs {
		hex, err := getSerializedInput(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	for _, idx := range inputScripts {
		hex, err := getInputScript(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	for _, idx := range inputPrevTxIDs {
		hex, err := getInputPrevTxID(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	for _, idx := range inputPrevOuts {
		hex, err := getInputPrevOut(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	for _, idx := range inputSequences {
		hex, err := getInputSequence(tx, idx)
		if err != nil {
			return err
		}
		fmt.Println(hex)
	}

	// Locktime (output last to match transaction order)
	if getLocktime {
		fmt.Println(encodeUint32LE(tx.LockTime))
	}

	return nil
}

// Output extraction functions

func getSerializedOutput(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Outputs) {
		return "", fmt.Errorf("output index %d out of range (0-%d)", idx, len(tx.Outputs)-1)
	}

	output := tx.Outputs[idx]
	bytes := output.Bytes()
	return hex.EncodeToString(bytes), nil
}

func getOutputScript(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Outputs) {
		return "", fmt.Errorf("output index %d out of range (0-%d)", idx, len(tx.Outputs)-1)
	}

	output := tx.Outputs[idx]
	if output.LockingScript == nil {
		return "", nil
	}
	return output.LockingScript.String(), nil
}

func getOutputValue(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Outputs) {
		return "", fmt.Errorf("output index %d out of range (0-%d)", idx, len(tx.Outputs)-1)
	}

	output := tx.Outputs[idx]
	return encodeUint64LE(output.Satoshis), nil
}

// Input extraction functions

func getSerializedInput(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("input index %d out of range (0-%d)", idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	bytes := input.Bytes(false) // false = don't include source tx info
	return hex.EncodeToString(bytes), nil
}

func getInputScript(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("input index %d out of range (0-%d)", idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	if input.UnlockingScript == nil {
		return "", nil
	}
	return input.UnlockingScript.String(), nil
}

func getInputPrevTxID(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("input index %d out of range (0-%d)", idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	if input.SourceTXID == nil {
		return "", fmt.Errorf("input %d has no previous txid", idx)
	}
	return input.SourceTXID.String(), nil
}

func getInputPrevOut(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("input index %d out of range (0-%d)", idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	return encodeUint32LE(input.SourceTxOutIndex), nil
}

func getInputSequence(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("input index %d out of range (0-%d)", idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	return encodeUint32LE(input.SequenceNumber), nil
}

// Encoding helpers

func encodeUint32LE(v uint32) string {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return hex.EncodeToString(buf)
}

func encodeUint64LE(v uint64) string {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return hex.EncodeToString(buf)
}

// init initializes the cobra command flags.
func init() {
	// Transaction input
	rootCmd.Flags().StringVarP(&raw, "raw", "r", "", "Raw transaction hex")

	// Output selectors
	rootCmd.Flags().IntSliceVarP(&outputs, "output", "o", nil, "Select complete serialized output at index (can repeat)")
	rootCmd.Flags().IntSliceVar(&outputScripts, "output-script", nil, "Select output locking script at index (can repeat)")
	rootCmd.Flags().IntSliceVar(&outputValues, "output-value", nil, "Select output value at index (can repeat)")

	// Input selectors
	rootCmd.Flags().IntSliceVarP(&inputs, "input", "i", nil, "Select complete serialized input at index (can repeat)")
	rootCmd.Flags().IntSliceVar(&inputScripts, "input-script", nil, "Select input unlocking script at index (can repeat)")
	rootCmd.Flags().IntSliceVar(&inputPrevTxIDs, "input-prevtxid", nil, "Select input previous txid at index (can repeat)")
	rootCmd.Flags().IntSliceVar(&inputPrevOuts, "input-prevout", nil, "Select input previous output index at index (can repeat)")
	rootCmd.Flags().IntSliceVar(&inputSequences, "input-sequence", nil, "Select input sequence number at index (can repeat)")

	// Transaction-level selectors
	rootCmd.Flags().BoolVarP(&getVersion, "version", "v", false, "Select transaction version (4-byte LE hex)")
	rootCmd.Flags().BoolVarP(&getLocktime, "locktime", "l", false, "Select transaction locktime (4-byte LE hex)")
	rootCmd.Flags().BoolVar(&getTxID, "txid", false, "Select transaction ID")
}

// main is the entry point for the pick command.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
