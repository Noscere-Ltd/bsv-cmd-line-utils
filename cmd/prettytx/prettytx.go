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

// Command-line flags
var (
	raw     string // Raw transaction hex provided via flag
	noColor bool   // Disable colored output
	compact bool   // Enable compact output mode
)

// rootCmd is the main cobra command for the prettytx tool.
var rootCmd = &cobra.Command{
	Use:   "prettytx",
	Short: "Parse and display Bitcoin transaction components",
	Long:  "A command line tool that parses raw Bitcoin transactions and displays their components in human-readable format",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

// run handles the main execution flow:
// 1. Reads transaction hex from flag or stdin
// 2. Validates the hex string
// 3. Parses and displays the transaction
func run() error {
	txString, err := getTransactionHex()
	if err != nil {
		return err
	}

	if txString == "" {
		return fmt.Errorf("no transaction provided")
	}

	// Check the string to ensure it is a hex string
	if !cli.IsValidHex(txString) {
		return fmt.Errorf("input is not a valid hex string")
	}

	// Parse and display transaction
	return parseTransaction(txString)
}

// getTransactionHex reads transaction hex from flag, stdin, or clipboard.
// Priority: 1) --raw flag, 2) stdin (if piped), 3) clipboard
func getTransactionHex() (string, error) {
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

// c applies ANSI color codes to text if color output is enabled.
// Returns plain text if --no-color flag is set, otherwise returns colorized text.
func c(color, text string) string {
	if noColor {
		return text
	}
	return color + text + colorReset
}

// parseTransaction decodes and displays a raw Bitcoin transaction in human-readable format.
func parseTransaction(rawTx string) error {
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
	printHeader(tx.TxID().String())
	printVersion(tx)
	printInputs(tx)
	printOutputs(tx)
	printLocktime(tx)
	printFooter(tx)

	return nil
}

// printHeader prints the transaction breakdown header.
func printHeader(txid string) {
	fmt.Printf("%s %s\n",
		c(colorDim, "TX ID:"),
		c(colorGreen, txid))
	fmt.Println(c(colorWhite, "────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────"))
}

// printVersion prints the transaction version.
func printVersion(tx *transaction.Transaction) {
	fmt.Printf("%s %d %s\n",
		c(colorDim, "Version:"),
		tx.Version,
		c(colorDim, fmt.Sprintf("(0x%08x)", tx.Version)))
}

// printInputs prints the transaction inputs section.
func printInputs(tx *transaction.Transaction) {
	inputCount := len(tx.Inputs)
	fmt.Printf("%s %d\n", c(colorDim, "Inputs:"), inputCount)

	if inputCount == 0 {
		return
	}

	for i, input := range tx.Inputs {
		printInput(i, input)
	}
}

// printInput prints a single transaction input.
func printInput(index int, input *transaction.TransactionInput) {
	fmt.Printf("\n%s\n", c(colorWhite, fmt.Sprintf("INPUT #%d", index)))

	// Previous transaction ID and output index on same line
	if input.SourceTXID != nil {
		fmt.Printf("  %s %s:%d\n",
			c(colorDim, "Prev:"),
			c(colorGreen, input.SourceTXID.String()),
			input.SourceTxOutIndex)
	} else {
		fmt.Printf("  %s %s\n",
			c(colorDim, "Prev:"),
			c(colorRed, "(null)"))
	}

	// Script
	printUnlockingScript(input.UnlockingScript)

	// Sequence number
	fmt.Printf("  %s %d %s\n",
		c(colorDim, "Sequence:"),
		input.SequenceNumber,
		c(colorDim, fmt.Sprintf("(0x%08x)", input.SequenceNumber)))
}

// truncateHex truncates a hex string if compact mode is enabled and it exceeds maxLen.
func truncateHex(hexStr string, maxLen int) string {
	if !compact || len(hexStr) <= maxLen {
		return hexStr
	}
	return hexStr[:maxLen] + "..."
}

// printUnlockingScript prints the unlocking script details for an input.
func printUnlockingScript(unlockingScript *script.Script) {
	if unlockingScript == nil {
		fmt.Printf("  %s %s\n", c(colorDim, "Script:"), c(colorDim, "(empty)"))
		return
	}

	scriptBytes := *unlockingScript
	scriptHex := scriptBytes.String()
	scriptLen := len(scriptBytes)

	fmt.Printf("  %s %s %s\n",
		c(colorDim, "Script:"),
		c(colorDim, truncateHex(scriptHex, 64)),
		c(colorDim, fmt.Sprintf("(%d bytes)", scriptLen)))

	// Try to extract address from P2PKH unlocking script
	addr := extractAddressFromUnlockingScript(unlockingScript, true)
	if addr != "" {
		fmt.Printf("  %s %s\n", c(colorDim, "Address:"), c(colorGreen, addr))
	}
}

// printOutputs prints the transaction outputs section.
func printOutputs(tx *transaction.Transaction) {
	outputCount := len(tx.Outputs)
	fmt.Printf("%s %d\n", c(colorDim, "Outputs:"), outputCount)

	if outputCount == 0 {
		return
	}

	for i, output := range tx.Outputs {
		printOutput(i, output)
	}
}

// printOutput prints a single transaction output.
func printOutput(index int, output *transaction.TransactionOutput) {
	fmt.Printf("\n%s\n", c(colorWhite, fmt.Sprintf("OUTPUT #%d", index)))

	// Value in satoshis
	satoshis := output.Satoshis
	btc := float64(satoshis) / 100000000.0
	fmt.Printf("  %s %s %s\n",
		c(colorDim, "Value:"),
		c(colorGreen, fmt.Sprintf("%d sats", satoshis)),
		c(colorDim, fmt.Sprintf("(%.8f BSV)", btc)))

	// Locking script
	printLockingScript(output.LockingScript)
}

// printLockingScript prints the locking script details for an output.
func printLockingScript(lockingScript *script.Script) {
	if lockingScript == nil {
		fmt.Printf("  %s %s\n", c(colorDim, "Script:"), c(colorDim, "(empty)"))
		return
	}

	scriptBytes := *lockingScript
	scriptHex := scriptBytes.String()
	scriptLen := len(scriptBytes)

	fmt.Printf("  %s %s %s\n",
		c(colorDim, "Script:"),
		c(colorDim, truncateHex(scriptHex, 64)),
		c(colorDim, fmt.Sprintf("(%d bytes)", scriptLen)))

	// Try to extract P2PKH address
	addr := extractP2PKHAddress(lockingScript, true)
	if addr != "" {
		fmt.Printf("  %s %s\n", c(colorDim, "Address:"), c(colorGreen, addr))
	}
}

// printLocktime prints the transaction locktime.
func printLocktime(tx *transaction.Transaction) {
	lockInfo := ""
	if tx.LockTime == 0 {
		lockInfo = "(Not locked)"
	} else if tx.LockTime < 500000000 {
		lockInfo = fmt.Sprintf("(Block %d)", tx.LockTime)
	} else {
		lockInfo = fmt.Sprintf("(Timestamp %d)", tx.LockTime)
	}

	fmt.Printf("\n%s %d %s\n",
		c(colorDim, "nLockTime:"),
		tx.LockTime,
		c(colorDim, lockInfo))
}

// printFooter prints the transaction footer with TXID.
func printFooter(tx *transaction.Transaction) {
	fmt.Println(c(colorWhite, "────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────"))
	fmt.Printf("%s %s\n",
		c(colorDim, "TX ID:"),
		c(colorGreen, tx.TxID().String()))
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

// extractAddressFromUnlockingScript attempts to extract an address from a P2PKH unlocking script.
// P2PKH unlocking scripts contain: <signature> <pubKey>
// This function extracts the public key and derives the address from it.
// Returns the address string if successful, empty string otherwise.
func extractAddressFromUnlockingScript(scriptBytes *script.Script, mainnet bool) string {
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
	addr, err := script.NewAddressFromPublicKey(pubKey, mainnet)
	if err != nil {
		return ""
	}

	return addr.AddressString
}

// extractPublicKeyFromScript parses a script to extract the public key.
// In a typical P2PKH unlocking script:
// - First comes the signature (variable length, typically ~72 bytes)
// - Then comes the public key (33 or 65 bytes)
func extractPublicKeyFromScript(bytes []byte) []byte {
	var pubKeyBytes []byte
	i := 0

	for i < len(bytes) {
		if i >= len(bytes) {
			break
		}

		opcode := bytes[i]
		i++

		// Handle push data opcodes
		if opcode > 0 && opcode <= 75 {
			// Direct push of N bytes
			length := int(opcode)
			if i+length > len(bytes) {
				break
			}
			data := bytes[i : i+length]
			i += length

			// Check if this looks like a public key (33 or 65 bytes)
			if length == 33 || length == 65 {
				pubKeyBytes = data
			}
		} else if opcode == 0x4c { // OP_PUSHDATA1
			if i >= len(bytes) {
				break
			}
			length := int(bytes[i])
			i++
			if i+length > len(bytes) {
				break
			}
			data := bytes[i : i+length]
			i += length

			if length == 33 || length == 65 {
				pubKeyBytes = data
			}
		}
	}

	return pubKeyBytes
}

// init initializes the cobra command flags.
// This function is automatically called by Go before main() executes.
func init() {
	rootCmd.Flags().StringVarP(&raw, "raw", "r", "", "Raw transaction hex to parse")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.Flags().BoolVarP(&compact, "compact", "c", false, "Enable compact output with truncated scripts")
}

// main is the entry point for the prettytx command.
// It executes the cobra root command which handles flag parsing and command execution.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
