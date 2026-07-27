// Package main implements a Bitcoin SV transaction builder with smart UTXO selection.
//
// NO SATOSHI LEFT BEHIND — every satoshi is accounted for. If there is change,
// it always gets its own output. No dust thresholds, no silent fee absorption.
//
// Features:
//   - Smart UTXO selection using largest-first algorithm
//   - Automatic fee estimation with 100 satoshi minimum floor
//   - Support for "send all" transactions (sats=0) — sends to destination address
//   - Split payments across multiple equal outputs with remainder handling
//   - Mainnet/testnet support via BananaBlocks, falling back to WhatsOnChain
//   - Debug mode for verbose logging
//   - Change output for every non-zero remainder (NO SATOSHI LEFT BEHIND)
//
// Usage:
//
//	carve -w <WIF> -a <address> -s 1000              # Send 1000 satoshis
//	carve -w <WIF> -a <address>                      # Send all funds to address
//	carve -w <WIF> -a <address> -s 1000 -t           # Use testnet
//	carve -w <WIF> -a <address> --debug              # Enable debug output
//	carve -w <WIF> -a <address> -f 200               # Custom fee rate
//	carve -w <WIF> -a <address> -s 1000000 -n 10     # Split 1M satoshis into 10 equal outputs
//	carve -w <WIF> -a <address> -s 1000001 -n 10     # Split into 10 outputs (9×100000 + 1×100001)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	"github.com/spf13/cobra"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/utxo"
)

// Transaction size estimation constants
const (
	inputSize  = 148 // Approximate bytes per input
	outputSize = 34  // Approximate bytes per output
	baseTxSize = 10  // Base transaction overhead
	minFee     = 100 // Minimum fee in satoshis
)

var (
	errWifAndAddressRequired = errors.New("--wif and --address are required")
	errSplitTooSmall         = errors.New("--split must be at least 1")
	errSplitRequiresAmount   = errors.New("--split requires a specific amount (--sats), cannot be used with send-all mode")
	errNoUTXOsForAddress     = errors.New("no UTXOs found for address")
	errNoAvailableUTXOs      = errors.New("no available UTXOs for address (all are spent in mempool)")
	errAPIError              = errors.New("API error")
	errNoUTXOsAvailable      = errors.New("no UTXOs available")
	errInsufficientFunds     = errors.New("insufficient funds")
)

// flags holds the command-line flags for the carve tool.
type flags struct {
	wif      string // WIF private key for signing
	address  string // Destination address
	sats     uint64 // Amount to send in satoshis (0 = send all)
	split    int    // Number of outputs to split the amount into (1 = no split)
	testnet  bool   // Use testnet instead of mainnet
	feePerKb uint64 // Fee rate in satoshis per kilobyte
	debug    bool   // Enable verbose debug logging
}

// newRootCmd builds the cobra command for the carve tool.
func newRootCmd() *cobra.Command {
	f := &flags{}

	cmd := &cobra.Command{
		Use:   "carve",
		Short: "Create and sign a BSV transaction from a WIF",
		Long:  "A command line tool that creates a signed transaction from a WIF private key, sending satoshis to a destination address",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateFlags(cmd, f); err != nil {
				return err
			}

			return carveTransaction(f)
		},
	}

	cmd.Flags().StringVarP(&f.wif, "wif", "w", "", "Source WIF private key (required)")
	cmd.Flags().StringVarP(&f.address, "address", "a", "", "Destination address (required)")
	cmd.Flags().Uint64VarP(&f.sats, "sats", "s", 0, "Amount in satoshis to send (default: 0 = send all minus fees)")
	cmd.Flags().IntVarP(&f.split, "split", "n", 1, "Number of equal outputs to split the amount into (default: 1 = no split)")
	cmd.Flags().BoolVarP(&f.testnet, "testnet", "t", false, "Use testnet")
	cmd.Flags().Uint64VarP(&f.feePerKb, "fee-per-kb", "f", 100, "Fee per kilobyte in satoshis")
	cmd.Flags().BoolVar(&f.debug, "debug", false, "Enable debug logging")

	if err := cmd.MarkFlagRequired("wif"); err != nil {
		panic(err)
	}

	if err := cmd.MarkFlagRequired("address"); err != nil {
		panic(err)
	}

	return cmd
}

// validateFlags checks that required flags are present and have valid values.
func validateFlags(cmd *cobra.Command, f *flags) error {
	if f.wif == "" || f.address == "" {
		_ = cmd.Help()
		return errWifAndAddressRequired
	}

	if f.split < 1 {
		_ = cmd.Help()
		return errSplitTooSmall
	}

	if f.split > 1 && f.sats == 0 {
		_ = cmd.Help()
		return errSplitRequiresAmount
	}

	return nil
}

// UTXO represents an unspent transaction output from the WhatsOnChain API.
type UTXO struct {
	TxHash string `json:"tx_hash"` // Transaction ID containing this output
	TxPos  uint32 `json:"tx_pos"`  // Output index (vout) within the transaction
	Value  uint64 `json:"value"`   // Value in satoshis
}

// carveTransaction is the main transaction creation workflow.
func carveTransaction(f *flags) error {
	ctx := context.Background()

	// 1. Derive private key and address from WIF
	privKey, sourceAddress, err := deriveKeyAndAddress(f)
	if err != nil {
		return err
	}

	// 2. Fetch UTXOs from WhatsOnChain
	utxos, err := fetchUTXOs(ctx, sourceAddress.AddressString, f)
	if err != nil {
		return err
	}

	// 3. Select appropriate UTXOs
	selectedUTXOs, err := selectAppropriateUTXOs(utxos, f)
	if err != nil {
		return err
	}

	// 4. Build the transaction
	tx, err := buildTransaction(privKey, sourceAddress, f.address, selectedUTXOs, f.sats, f.split, f.feePerKb, f.debug)
	if err != nil {
		return fmt.Errorf("failed to build transaction: %w", err)
	}

	// 5. Output the raw transaction hex to stdout
	if _, err := fmt.Fprintln(os.Stdout, tx.String()); err != nil {
		return err
	}

	return nil
}

// deriveKeyAndAddress parses the WIF and derives the source address.
func deriveKeyAndAddress(f *flags) (*ec.PrivateKey, *script.Address, error) {
	privKey, err := ec.PrivateKeyFromWif(f.wif)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse WIF: %w", err)
	}

	if f.debug {
		log.Printf("Testnet: %t", f.testnet)
	}

	// Derive the source address from the private key
	// NewAddressFromPublicKey takes mainnet bool, not testnet bool
	sourceAddress, err := script.NewAddressFromPublicKey(privKey.PubKey(), !f.testnet)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive source address: %w", err)
	}

	if f.debug {
		log.Printf("Source address: %s", sourceAddress.AddressString)
	}

	return privKey, sourceAddress, nil
}

// fetchUTXOs retrieves UTXOs from WhatsOnChain and validates them.
func fetchUTXOs(ctx context.Context, addr string, f *flags) ([]*UTXO, error) {
	utxos, err := getUnspentOutputs(ctx, addr, f)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch UTXOs: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("%w %s", errNoUTXOsForAddress, addr)
	}

	if f.debug {
		log.Printf("Found %d UTXO(s)", len(utxos))
	}

	return utxos, nil
}

// selectAppropriateUTXOs selects UTXOs based on the target amount.
func selectAppropriateUTXOs(utxos []*UTXO, f *flags) ([]*UTXO, error) {
	if f.sats == 0 {
		// Send all funds - use all UTXOs
		if f.debug {
			log.Printf("Sending all available funds")
		}

		return utxos, nil
	}

	// Select minimum UTXOs needed to cover the amount
	selected, err := selectUTXOs(utxos, f.sats, f.feePerKb, f.debug)
	if err != nil {
		return nil, fmt.Errorf("UTXO selection failed: %w", err)
	}

	return selected, nil
}

// getUnspentOutputs fetches all unspent transaction outputs (UTXOs) for a given address.
func getUnspentOutputs(ctx context.Context, addr string, f *flags) ([]*UTXO, error) {
	body, err := utxo.Fetch(ctx, addr, f.testnet, f.debug)
	if err != nil {
		return nil, err
	}

	utxos, err := parseUTXOResponse(body)
	if err != nil {
		return nil, err
	}

	// Filter and deduplicate
	return filterAndDeduplicateUTXOs(utxos, addr, f.debug)
}

// WOCUnspent represents a single UTXO from WhatsOnChain API.
type WOCUnspent struct {
	Height             int    `json:"height"`
	TxPos              int    `json:"tx_pos"`
	TxHash             string `json:"tx_hash"`
	Value              uint64 `json:"value"`
	IsSpentInMempoolTx bool   `json:"isSpentInMempoolTx"`
	Status             string `json:"status"`
}

// WOCUnspentAllResponse is the response structure from /unspent/all endpoint.
type WOCUnspentAllResponse struct {
	Address string       `json:"address"`
	Script  string       `json:"script"`
	Result  []WOCUnspent `json:"result"`
	Error   string       `json:"error"`
}

// parseUTXOResponse parses the WhatsOnChain API response.
func parseUTXOResponse(body []byte) ([]*UTXO, error) {
	var response WOCUnspentAllResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse UTXOs: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("%w: %s", errAPIError, response.Error)
	}

	utxos := make([]*UTXO, 0, len(response.Result))
	for _, u := range response.Result {
		// Skip UTXOs that are already spent in mempool
		if u.IsSpentInMempoolTx {
			continue
		}

		utxos = append(utxos, &UTXO{
			TxHash: u.TxHash,
			TxPos:  uint32(u.TxPos), //nolint:gosec // G115: TxPos is a transaction output index, safe to convert
			Value:  u.Value,
		})
	}

	return utxos, nil
}

// filterAndDeduplicateUTXOs removes spent and duplicate UTXOs.
func filterAndDeduplicateUTXOs(utxos []*UTXO, addr string, debug bool) ([]*UTXO, error) {
	if len(utxos) == 0 {
		return nil, fmt.Errorf("%w %s", errNoUTXOsForAddress, addr)
	}

	// Deduplicate by txid:vout
	seen := make(map[string]bool)
	dedupedUTXOs := make([]*UTXO, 0, len(utxos))

	for _, utxo := range utxos {
		key := fmt.Sprintf("%s:%d", utxo.TxHash, utxo.TxPos)

		if !seen[key] {
			seen[key] = true

			dedupedUTXOs = append(dedupedUTXOs, utxo)
		} else if debug {
			log.Printf("  Skipping duplicate UTXO: %s", key)
		}
	}

	if len(dedupedUTXOs) < len(utxos) && debug {
		log.Printf("Removed %d duplicate UTXO(s)", len(utxos)-len(dedupedUTXOs))
	}

	if len(dedupedUTXOs) == 0 {
		return nil, fmt.Errorf("%w %s", errNoAvailableUTXOs, addr)
	}

	return dedupedUTXOs, nil
}

// selectUTXOs implements a largest-first UTXO selection algorithm.
func selectUTXOs(utxos []*UTXO, targetAmount, feePerKb uint64, debug bool) ([]*UTXO, error) {
	if len(utxos) == 0 {
		return nil, errNoUTXOsAvailable
	}

	// Sort UTXOs by value (largest first)
	sortedUTXOs := make([]*UTXO, len(utxos))
	copy(sortedUTXOs, utxos)
	sort.Slice(sortedUTXOs, func(i, j int) bool {
		return sortedUTXOs[i].Value > sortedUTXOs[j].Value
	})

	selected := make([]*UTXO, 0, len(sortedUTXOs))

	var totalValue uint64

	for _, utxo := range sortedUTXOs {
		selected = append(selected, utxo)
		totalValue += utxo.Value

		// Calculate estimated fee with current number of inputs
		estimatedFee := calculateFee(len(selected), 2, feePerKb) // 2 outputs: payment + change

		// Check if we have enough to cover target amount + fee
		if totalValue >= targetAmount+estimatedFee {
			if debug {
				log.Printf("Selected %d UTXO(s) totaling %d satoshis (target: %d + fee: ~%d)",
					len(selected), totalValue, targetAmount, estimatedFee)
			}

			return selected, nil
		}
	}

	// Not enough funds
	estimatedFee := calculateFee(len(selected), 2, feePerKb)

	return nil, fmt.Errorf("%w: have %d satoshis, need %d (amount: %d + fee: ~%d)",
		errInsufficientFunds, totalValue, targetAmount+estimatedFee, targetAmount, estimatedFee)
}

// calculateFee estimates the transaction fee based on size.
func calculateFee(numInputs, numOutputs int, feePerKb uint64) uint64 {
	estimatedSize := uint64(numInputs*inputSize + numOutputs*outputSize + baseTxSize) //nolint:gosec // G115: size is always non-negative
	fee := (estimatedSize * feePerKb) / 1000

	// Enforce minimum fee
	if fee < minFee {
		fee = minFee
	}

	return fee
}

// buildTransaction constructs and signs a BSV transaction.
func buildTransaction(privKey *ec.PrivateKey, sourceAddr *script.Address, destAddrStr string, utxos []*UTXO, amount uint64, numOutputs int, feePerKb uint64, debug bool) (*transaction.Transaction, error) {
	// Create a new transaction
	tx := transaction.NewTransaction()

	// Parse destination address
	destAddr, err := script.NewAddressFromString(destAddrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}

	// Create P2PKH unlocker for signing
	unlocker, err := p2pkh.Unlock(privKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create unlocker: %w", err)
	}

	// Add all UTXOs as inputs
	totalInput, err := addInputs(tx, utxos, sourceAddr, unlocker, debug)
	if err != nil {
		return nil, err
	}

	// Add payment outputs
	if amount > 0 {
		if err := addPaymentOutputs(tx, destAddr, destAddrStr, amount, numOutputs, debug); err != nil {
			return nil, err
		}
	}

	// Calculate fee and add change output.
	// For send-all (amount == 0), remaining funds go to the DESTINATION address.
	// For normal sends, change goes back to the SOURCE address.
	changeAddr := sourceAddr
	if amount == 0 {
		changeAddr = destAddr
	}

	if err := addChangeOutput(tx, changeAddr, totalInput, amount, feePerKb, debug); err != nil {
		return nil, err
	}

	// Sign all inputs
	if err := tx.Sign(); err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	if debug {
		log.Printf("Transaction ID: %s", tx.TxID().String())
	}

	return tx, nil
}

// addInputs adds all UTXOs as transaction inputs.
func addInputs(tx *transaction.Transaction, utxos []*UTXO, sourceAddr *script.Address, unlocker *p2pkh.P2PKH, debug bool) (uint64, error) {
	var totalInput uint64

	for _, utxo := range utxos {
		// Create the locking script from the source address (P2PKH)
		lockingScript, err := p2pkh.Lock(sourceAddr)
		if err != nil {
			return 0, fmt.Errorf("failed to create locking script: %w", err)
		}

		// Add input
		err = tx.AddInputFrom(
			utxo.TxHash,
			utxo.TxPos,
			lockingScript.String(),
			utxo.Value,
			unlocker,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to add input: %w", err)
		}

		totalInput += utxo.Value
	}

	if debug {
		log.Printf("Total input: %d satoshis", totalInput)
	}

	return totalInput, nil
}

// addPaymentOutputs adds payment outputs to the destination address.
func addPaymentOutputs(tx *transaction.Transaction, destAddr *script.Address, destAddrStr string, amount uint64, numOutputs int, debug bool) error {
	destLockingScript, err := p2pkh.Lock(destAddr)
	if err != nil {
		return fmt.Errorf("failed to create destination locking script: %w", err)
	}

	if numOutputs < 1 {
		numOutputs = 1
	}

	// Calculate amount per output and remainder
	amountPerOutput := amount / uint64(numOutputs) //nolint:gosec // G115: numOutputs is guarded to be >= 1 above
	remainder := amount % uint64(numOutputs)       //nolint:gosec // G115: numOutputs is guarded to be >= 1 above

	// Add outputs with equal amounts
	for i := 0; i < numOutputs; i++ {
		outputAmount := amountPerOutput

		// Add remainder to the last output
		if i == numOutputs-1 {
			outputAmount += remainder
		}

		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      outputAmount,
			LockingScript: destLockingScript,
		})

		if debug {
			log.Printf("Output %d to %s: %d satoshis", i+1, destAddrStr, outputAmount)
		}
	}

	if debug && remainder > 0 {
		log.Printf("Remainder of %d satoshis added to last output", remainder)
	}

	return nil
}

// addChangeOutput calculates fees and adds a change output if needed.
// NO SATOSHI LEFT BEHIND: if change > 0, always create a change output.
func addChangeOutput(tx *transaction.Transaction, changeAddr *script.Address, totalInput, amount, feePerKb uint64, debug bool) error {
	// Calculate fees
	estimatedSize := uint64(len(tx.Inputs)*inputSize + len(tx.Outputs)*outputSize + baseTxSize) //nolint:gosec // G115: len() results are non-negative
	fee := (estimatedSize * feePerKb) / 1000

	// Add extra for the change output size
	fee += uint64(outputSize) * feePerKb / 1000

	// Enforce minimum fee
	if fee < minFee {
		fee = minFee
	}

	if debug {
		log.Printf("Estimated size: %d bytes, Fee: %d satoshis", estimatedSize, fee)
	}

	change := totalInput - amount - fee

	if change > 0 {
		changeLockingScript, err := p2pkh.Lock(changeAddr)
		if err != nil {
			return fmt.Errorf("failed to create change locking script: %w", err)
		}

		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      change,
			LockingScript: changeLockingScript,
		})

		if debug {
			log.Printf("Change to %s: %d satoshis", changeAddr.AddressString, change)
		}
	}

	return nil
}

// main is the entry point for the carve command.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
