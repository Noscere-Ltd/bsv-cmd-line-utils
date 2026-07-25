// Package main implements the opreturn CLI tool for building and signing BSV OP_RETURN transactions.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	"github.com/spf13/cobra"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
)

// Transaction size estimation constants (same as carve)
const (
	inputSize  = 148
	outputSize = 34
	baseTxSize = 10
	minFee     = 100
)

var (
	errWifRequired       = errors.New("--wif is required")
	errNoDataProvided    = errors.New("no data provided")
	errNoUTXOsForAddress = errors.New("no UTXOs found for address")
	errWhatsOnChainAPI   = errors.New("WhatsOnChain API error")
	errAPIError          = errors.New("API error")
	errNoUTXOsAvailable  = errors.New("no UTXOs available")
	errInsufficientFunds = errors.New("insufficient funds")
)

// opreturnFlags holds the CLI flag values for the opreturn command.
type opreturnFlags struct {
	wif       string
	testnet   bool
	feePerKb  uint64
	dustLimit uint64
	debug     bool
}

func newRootCmd() *cobra.Command {
	var flags opreturnFlags

	cmd := &cobra.Command{
		Use:   "opreturn [data...]",
		Short: "Create a transaction with an OP_RETURN output",
		Long:  "A command line tool that creates a signed BSV transaction with an OP_RETURN data output. Multiple arguments become multiple pushdata parts. Outputs raw tx hex to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.wif == "" {
				cmd.Help() //nolint:errcheck,gosec // help output failure is not actionable here
				return errWifRequired
			}

			return buildOpReturn(args, flags)
		},
	}

	cmd.Flags().StringVarP(&flags.wif, "wif", "w", "", "WIF private key for signing (required)")
	cmd.Flags().BoolVarP(&flags.testnet, "testnet", "t", false, "Use testnet")
	cmd.Flags().Uint64VarP(&flags.feePerKb, "fee-per-kb", "f", 100, "Fee per kilobyte in satoshis")
	cmd.Flags().Uint64VarP(&flags.dustLimit, "dust", "d", 1, "Dust limit in satoshis")
	cmd.Flags().BoolVar(&flags.debug, "debug", false, "Enable debug logging")

	if err := cmd.MarkFlagRequired("wif"); err != nil {
		panic(err)
	}

	return cmd
}

func buildOpReturn(args []string, flags opreturnFlags) error {
	ctx := context.Background()

	parts, err := getDataParts(args)
	if err != nil {
		return err
	}

	if len(parts) == 0 {
		return errNoDataProvided
	}

	privKey, sourceAddr, selected, err := prepareInputs(ctx, flags)
	if err != nil {
		return err
	}

	tx := transaction.NewTransaction()

	totalInput, err := addInputs(tx, selected, sourceAddr, privKey)
	if err != nil {
		return err
	}

	if err = addOpReturnOutputs(tx, parts); err != nil {
		return err
	}

	fee, err := addChangeOutput(tx, sourceAddr, totalInput, flags)
	if err != nil {
		return err
	}

	if flags.debug {
		log.Printf("Total input: %d sats, Estimated fee: %d sats", totalInput, fee)
	}

	return signAndPrint(tx, flags.debug)
}

// signAndPrint signs tx and writes its raw hex to stdout.
func signAndPrint(tx *transaction.Transaction, debug bool) error {
	if err := tx.Sign(); err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	if debug {
		log.Printf("Transaction ID: %s", tx.TxID().String())
	}

	if _, err := fmt.Fprintln(os.Stdout, tx.String()); err != nil {
		return err
	}

	return nil
}

// prepareInputs derives the signing key and address from the WIF flag, then fetches and
// selects enough UTXOs to cover the OP_RETURN transaction's fee.
func prepareInputs(ctx context.Context, flags opreturnFlags) (*ec.PrivateKey, *script.Address, []*UTXO, error) {
	privKey, err := ec.PrivateKeyFromWif(flags.wif)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse WIF: %w", err)
	}

	sourceAddr, err := script.NewAddressFromPublicKey(privKey.PubKey(), !flags.testnet)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to derive address: %w", err)
	}

	if flags.debug {
		log.Printf("Source address: %s", sourceAddr.AddressString)
	}

	utxos, err := getUnspentOutputs(ctx, sourceAddr.AddressString, flags.testnet, flags.debug)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to fetch UTXOs: %w", err)
	}

	if len(utxos) == 0 {
		return nil, nil, nil, fmt.Errorf("%w: %s", errNoUTXOsForAddress, sourceAddr.AddressString)
	}

	if flags.debug {
		log.Printf("Found %d UTXO(s)", len(utxos))
	}

	// Select UTXOs (target amount = 0, just need fee coverage)
	selected, err := selectUTXOs(utxos, 0, flags.feePerKb, flags.debug)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("UTXO selection failed: %w", err)
	}

	return privKey, sourceAddr, selected, nil
}

// addInputs adds the selected UTXOs as signed inputs to tx and returns the total input value.
func addInputs(tx *transaction.Transaction, selected []*UTXO, sourceAddr *script.Address, privKey *ec.PrivateKey) (uint64, error) {
	unlocker, err := p2pkh.Unlock(privKey, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create unlocker: %w", err)
	}

	var totalInput uint64

	for _, utxo := range selected {
		lockingScript, err := p2pkh.Lock(sourceAddr)
		if err != nil {
			return 0, fmt.Errorf("failed to create locking script: %w", err)
		}

		if err := tx.AddInputFrom(utxo.TxHash, utxo.TxPos, lockingScript.String(), utxo.Value, unlocker); err != nil {
			return 0, fmt.Errorf("failed to add input: %w", err)
		}

		totalInput += utxo.Value
	}

	return totalInput, nil
}

// addOpReturnOutputs adds the data parts to tx as a single or multi-part OP_RETURN output.
func addOpReturnOutputs(tx *transaction.Transaction, parts [][]byte) error {
	if len(parts) == 1 {
		if err := tx.AddOpReturnOutput(parts[0]); err != nil {
			return fmt.Errorf("failed to add OP_RETURN output: %w", err)
		}

		return nil
	}

	if err := tx.AddOpReturnPartsOutput(parts); err != nil {
		return fmt.Errorf("failed to add OP_RETURN output: %w", err)
	}

	return nil
}

// addChangeOutput calculates the fee and, if the change exceeds the dust limit, adds a change
// output back to sourceAddr. It returns the estimated fee.
func addChangeOutput(tx *transaction.Transaction, sourceAddr *script.Address, totalInput uint64, flags opreturnFlags) (uint64, error) {
	estimatedSize := uint64(len(tx.Inputs)*inputSize + (len(tx.Outputs)+1)*outputSize + baseTxSize)

	fee := (estimatedSize * flags.feePerKb) / 1000
	if fee < minFee {
		fee = minFee
	}

	change := totalInput - fee
	if change > flags.dustLimit {
		changeLockingScript, err := p2pkh.Lock(sourceAddr)
		if err != nil {
			return fee, fmt.Errorf("failed to create change locking script: %w", err)
		}

		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      change,
			LockingScript: changeLockingScript,
		})

		if flags.debug {
			log.Printf("Change: %d sats", change)
		}
	} else if flags.debug {
		log.Printf("Change (%d sats) below dust limit, adding to fee", change)
	}

	return fee, nil
}

func getDataParts(args []string) ([][]byte, error) {
	if len(args) > 0 {
		parts := make([][]byte, len(args))
		for i, a := range args {
			parts[i] = []byte(a)
		}

		return parts, nil
	}

	// Try stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		msg, err := cli.ReadTextFromReader(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}

		if msg != "" {
			return [][]byte{[]byte(msg)}, nil
		}
	}

	return nil, nil
}

// UTXO represents an unspent transaction output.
type UTXO struct {
	TxHash string `json:"tx_hash"`
	TxPos  uint32 `json:"tx_pos"`
	Value  uint64 `json:"value"`
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

func getUnspentOutputs(ctx context.Context, addr string, testnet, debug bool) ([]*UTXO, error) {
	response, err := fetchUnspentResponse(ctx, addr, testnet, debug)
	if err != nil {
		return nil, err
	}

	if response.Error != "" {
		return nil, fmt.Errorf("%w: %s", errAPIError, response.Error)
	}

	return filterUnspent(response.Result), nil
}

// fetchUnspentResponse calls the WhatsOnChain unspent/all endpoint for addr and decodes the response.
func fetchUnspentResponse(ctx context.Context, addr string, testnet, debug bool) (*WOCUnspentAllResponse, error) {
	network := "main"
	if testnet {
		network = "test"
	}

	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent/all", network, addr)

	if debug {
		log.Printf("Fetching UTXOs from WhatsOnChain (%s)...", network)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch UTXOs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w (status %d): %s", errWhatsOnChainAPI, resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response WOCUnspentAllResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse UTXOs: %w", err)
	}

	return &response, nil
}

// filterUnspent drops mempool-spent entries and duplicates from result.
func filterUnspent(result []WOCUnspent) []*UTXO {
	seen := make(map[string]bool)

	var utxos []*UTXO

	for _, u := range result {
		if u.IsSpentInMempoolTx || u.TxPos < 0 {
			continue
		}

		key := fmt.Sprintf("%s:%d", u.TxHash, u.TxPos)
		if seen[key] {
			continue
		}

		seen[key] = true

		utxos = append(utxos, &UTXO{
			TxHash: u.TxHash,
			TxPos:  uint32(u.TxPos), //nolint:gosec // guarded by u.TxPos < 0 check above
			Value:  u.Value,
		})
	}

	return utxos
}

func selectUTXOs(utxos []*UTXO, targetAmount, feeRate uint64, debug bool) ([]*UTXO, error) {
	if len(utxos) == 0 {
		return nil, errNoUTXOsAvailable
	}

	sorted := make([]*UTXO, len(utxos))
	copy(sorted, utxos)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	var selected []*UTXO

	var totalValue uint64

	for _, utxo := range sorted {
		selected = append(selected, utxo)
		totalValue += utxo.Value

		estimatedFee := calculateFee(len(selected), 2, feeRate) // OP_RETURN + change
		if totalValue >= targetAmount+estimatedFee {
			if debug {
				log.Printf("Selected %d UTXO(s) totaling %d sats", len(selected), totalValue)
			}

			return selected, nil
		}
	}

	estimatedFee := calculateFee(len(selected), 2, feeRate)

	return nil, fmt.Errorf("%w: have %d sats, need %d (amount: %d + fee: ~%d)",
		errInsufficientFunds, totalValue, targetAmount+estimatedFee, targetAmount, estimatedFee)
}

func calculateFee(numInputs, numOutputs int, feeRate uint64) uint64 {
	size := uint64(numInputs*inputSize + numOutputs*outputSize + baseTxSize) //nolint:gosec // numInputs/numOutputs are non-negative lengths

	fee := (size * feeRate) / 1000
	if fee < minFee {
		fee = minFee
	}

	return fee
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
