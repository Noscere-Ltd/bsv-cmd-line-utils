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
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/spf13/cobra"
)

var (
	errNoSelectorSpecified   = errors.New("no selector specified")
	errNoTransactionProvided = errors.New("no transaction provided")
	errNotValidHex           = errors.New("input is not a valid hex string")
	errHTTPError             = errors.New("HTTP error")
	errOutputIndexOutOfRange = errors.New("output index out of range")
	errInputIndexOutOfRange  = errors.New("input index out of range")
	errInputHasNoPrevTxID    = errors.New("input has no previous txid")
)

// pickFlags holds the command-line selectors for the pick tool.
type pickFlags struct {
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
}

// newRootCmd builds the pick cobra command.
func newRootCmd() *cobra.Command {
	flags := &pickFlags{}

	cmd := &cobra.Command{
		Use:   "pick [rawtx]",
		Short: "Extract parts from a Bitcoin transaction",
		Long: `A command line tool that extracts specific parts from raw Bitcoin transactions
and outputs them as hex strings for pipeline processing.

Supports selecting outputs, inputs, and transaction-level fields.
Multiple selections can be combined in one call.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args, flags)
		},
	}

	// Transaction input
	cmd.Flags().StringVarP(&flags.raw, "raw", "r", "", "Raw transaction hex")

	// Output selectors
	cmd.Flags().IntSliceVarP(&flags.outputs, "output", "o", nil, "Select complete serialized output at index (can repeat)")
	cmd.Flags().IntSliceVar(&flags.outputScripts, "output-script", nil, "Select output locking script at index (can repeat)")
	cmd.Flags().IntSliceVar(&flags.outputValues, "output-value", nil, "Select output value at index (can repeat)")

	// Input selectors
	cmd.Flags().IntSliceVarP(&flags.inputs, "input", "i", nil, "Select complete serialized input at index (can repeat)")
	cmd.Flags().IntSliceVar(&flags.inputScripts, "input-script", nil, "Select input unlocking script at index (can repeat)")
	cmd.Flags().IntSliceVar(&flags.inputPrevTxIDs, "input-prevtxid", nil, "Select input previous txid at index (can repeat)")
	cmd.Flags().IntSliceVar(&flags.inputPrevOuts, "input-prevout", nil, "Select input previous output index at index (can repeat)")
	cmd.Flags().IntSliceVar(&flags.inputSequences, "input-sequence", nil, "Select input sequence number at index (can repeat)")

	// Transaction-level selectors
	cmd.Flags().BoolVarP(&flags.getVersion, "version", "v", false, "Select transaction version (4-byte LE hex)")
	cmd.Flags().BoolVarP(&flags.getLocktime, "locktime", "l", false, "Select transaction locktime (4-byte LE hex)")
	cmd.Flags().BoolVar(&flags.getTxID, "txid", false, "Select transaction ID")

	return cmd
}

// run handles the main execution flow.
func run(cmd *cobra.Command, args []string, flags *pickFlags) error {
	// Check if any selector was specified
	if !hasAnySelector(flags) {
		_ = cmd.Help()
		return errNoSelectorSpecified
	}

	// Get transaction hex
	txHex, err := getTransactionHex(args, flags)
	if err != nil {
		return err
	}

	if txHex == "" {
		_ = cmd.Help()
		return errNoTransactionProvided
	}

	// Validate hex
	if !cli.IsValidHex(txHex) {
		return errNotValidHex
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
	return extractAndOutput(tx, flags)
}

// hasOutputSelector reports whether any output selector was provided.
func hasOutputSelector(flags *pickFlags) bool {
	return len(flags.outputs) > 0 || len(flags.outputScripts) > 0 || len(flags.outputValues) > 0
}

// hasInputSelector reports whether any input selector was provided.
func hasInputSelector(flags *pickFlags) bool {
	return len(flags.inputs) > 0 ||
		len(flags.inputScripts) > 0 ||
		len(flags.inputPrevTxIDs) > 0 ||
		len(flags.inputPrevOuts) > 0 ||
		len(flags.inputSequences) > 0
}

// hasFieldSelector reports whether any transaction-level field selector was provided.
func hasFieldSelector(flags *pickFlags) bool {
	return flags.getVersion || flags.getLocktime || flags.getTxID
}

// hasAnySelector checks if any selection flag was provided.
func hasAnySelector(flags *pickFlags) bool {
	return hasOutputSelector(flags) || hasInputSelector(flags) || hasFieldSelector(flags)
}

// getTransactionHex reads transaction hex from argument, flag, stdin, or file URL.
func getTransactionHex(args []string, flags *pickFlags) (string, error) {
	// Check argument first
	if len(args) > 0 {
		return resolveInput(args[0])
	}

	// Check flag
	if flags.raw != "" {
		return resolveInput(flags.raw)
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
		return resolveFileInput(path)
	}

	// Check if it's an HTTP/HTTPS URL
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return resolveHTTPInput(input)
	}

	// It's a raw hex string
	return input, nil
}

// resolveFileInput reads transaction hex from a local file.
func resolveFileInput(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is user-supplied CLI input by design
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	return cli.CleanString(string(data)), nil
}

// resolveHTTPInput fetches transaction hex from an HTTP/HTTPS URL.
func resolveHTTPInput(url string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching URL: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %d", errHTTPError, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	return cli.CleanString(string(data)), nil
}

// extractAndOutput extracts selected elements and prints them to stdout.
func extractAndOutput(tx *transaction.Transaction, flags *pickFlags) error {
	if err := extractFields(tx, flags); err != nil {
		return err
	}

	if err := extractOutputs(tx, flags); err != nil {
		return err
	}

	if err := extractInputs(tx, flags); err != nil {
		return err
	}

	// Locktime (output last to match transaction order)
	if flags.getLocktime {
		if _, err := fmt.Fprintln(os.Stdout, encodeUint32LE(tx.LockTime)); err != nil {
			return err
		}
	}

	return nil
}

// extractFields prints the transaction-level fields selected via flags.
func extractFields(tx *transaction.Transaction, flags *pickFlags) error {
	if flags.getVersion {
		if _, err := fmt.Fprintln(os.Stdout, encodeUint32LE(tx.Version)); err != nil {
			return err
		}
	}

	if flags.getTxID {
		if _, err := fmt.Fprintln(os.Stdout, tx.TxID().String()); err != nil {
			return err
		}
	}

	return nil
}

// extractOutputs prints the output selections in flag order.
func extractOutputs(tx *transaction.Transaction, flags *pickFlags) error {
	extractors := []struct {
		indices []int
		get     func(*transaction.Transaction, int) (string, error)
	}{
		{flags.outputs, getSerializedOutput},
		{flags.outputScripts, getOutputScript},
		{flags.outputValues, getOutputValue},
	}

	for _, e := range extractors {
		if err := printSelections(tx, e.indices, e.get); err != nil {
			return err
		}
	}

	return nil
}

// extractInputs prints the input selections in flag order.
func extractInputs(tx *transaction.Transaction, flags *pickFlags) error {
	extractors := []struct {
		indices []int
		get     func(*transaction.Transaction, int) (string, error)
	}{
		{flags.inputs, getSerializedInput},
		{flags.inputScripts, getInputScript},
		{flags.inputPrevTxIDs, getInputPrevTxID},
		{flags.inputPrevOuts, getInputPrevOut},
		{flags.inputSequences, getInputSequence},
	}

	for _, e := range extractors {
		if err := printSelections(tx, e.indices, e.get); err != nil {
			return err
		}
	}

	return nil
}

// printSelections prints the result of get for each requested index.
func printSelections(tx *transaction.Transaction, indices []int, get func(*transaction.Transaction, int) (string, error)) error {
	for _, idx := range indices {
		hex, err := get(tx, idx)
		if err != nil {
			return err
		}

		if _, err := fmt.Fprintln(os.Stdout, hex); err != nil {
			return err
		}
	}

	return nil
}

// Output extraction functions

func getSerializedOutput(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Outputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errOutputIndexOutOfRange, idx, len(tx.Outputs)-1)
	}

	output := tx.Outputs[idx]
	bytes := output.Bytes()

	return hex.EncodeToString(bytes), nil
}

func getOutputScript(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Outputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errOutputIndexOutOfRange, idx, len(tx.Outputs)-1)
	}

	output := tx.Outputs[idx]
	if output.LockingScript == nil {
		return "", nil
	}

	return output.LockingScript.String(), nil
}

func getOutputValue(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Outputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errOutputIndexOutOfRange, idx, len(tx.Outputs)-1)
	}

	output := tx.Outputs[idx]

	return encodeUint64LE(output.Satoshis), nil
}

// Input extraction functions

func getSerializedInput(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errInputIndexOutOfRange, idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	bytes := input.Bytes(false) // false = don't include source tx info

	return hex.EncodeToString(bytes), nil
}

func getInputScript(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errInputIndexOutOfRange, idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	if input.UnlockingScript == nil {
		return "", nil
	}

	return input.UnlockingScript.String(), nil
}

func getInputPrevTxID(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errInputIndexOutOfRange, idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]
	if input.SourceTXID == nil {
		return "", fmt.Errorf("%w: %d", errInputHasNoPrevTxID, idx)
	}

	return input.SourceTXID.String(), nil
}

func getInputPrevOut(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errInputIndexOutOfRange, idx, len(tx.Inputs)-1)
	}

	input := tx.Inputs[idx]

	return encodeUint32LE(input.SourceTxOutIndex), nil
}

func getInputSequence(tx *transaction.Transaction, idx int) (string, error) {
	if idx < 0 || idx >= len(tx.Inputs) {
		return "", fmt.Errorf("%w %d (0-%d)", errInputIndexOutOfRange, idx, len(tx.Inputs)-1)
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

// main is the entry point for the pick command.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
