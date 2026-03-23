package main

import (
	"context"
	"encoding/json"
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

	"github.com/n0sc/bsv-cmd-line-utils/internal/cli"
)

// Transaction size estimation constants (same as carve)
const (
	inputSize  = 148
	outputSize = 34
	baseTxSize = 10
	minFee     = 100
)

var (
	wifFlag   string
	testnet   bool
	feePerKb  uint64
	dustLimit uint64
	debug     bool
)

var rootCmd = &cobra.Command{
	Use:   "opreturn [data...]",
	Short: "Create a transaction with an OP_RETURN output",
	Long:  "A command line tool that creates a signed BSV transaction with an OP_RETURN data output. Multiple arguments become multiple pushdata parts. Outputs raw tx hex to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wifFlag == "" {
			cmd.Help() //nolint:errcheck
			return fmt.Errorf("--wif is required")
		}
		return buildOpReturn(args)
	},
}

func buildOpReturn(args []string) error {
	ctx := context.Background()

	// Get data payloads
	parts, err := getDataParts(args)
	if err != nil {
		return err
	}

	if len(parts) == 0 {
		return fmt.Errorf("no data provided")
	}

	// Parse WIF and derive address
	privKey, err := ec.PrivateKeyFromWif(wifFlag)
	if err != nil {
		return fmt.Errorf("failed to parse WIF: %w", err)
	}

	sourceAddr, err := script.NewAddressFromPublicKey(privKey.PubKey(), !testnet)
	if err != nil {
		return fmt.Errorf("failed to derive address: %w", err)
	}

	if debug {
		log.Printf("Source address: %s", sourceAddr.AddressString)
	}

	// Fetch UTXOs
	utxos, err := getUnspentOutputs(ctx, sourceAddr.AddressString)
	if err != nil {
		return fmt.Errorf("failed to fetch UTXOs: %w", err)
	}

	if len(utxos) == 0 {
		return fmt.Errorf("no UTXOs found for address %s", sourceAddr.AddressString)
	}

	if debug {
		log.Printf("Found %d UTXO(s)", len(utxos))
	}

	// Select UTXOs (target amount = 0, just need fee coverage)
	selected, err := selectUTXOs(utxos, 0, feePerKb)
	if err != nil {
		return fmt.Errorf("UTXO selection failed: %w", err)
	}

	// Build transaction
	tx := transaction.NewTransaction()

	// Create unlocker
	unlocker, err := p2pkh.Unlock(privKey, nil)
	if err != nil {
		return fmt.Errorf("failed to create unlocker: %w", err)
	}

	// Add inputs
	var totalInput uint64
	for _, utxo := range selected {
		lockingScript, err := p2pkh.Lock(sourceAddr)
		if err != nil {
			return fmt.Errorf("failed to create locking script: %w", err)
		}
		err = tx.AddInputFrom(utxo.TxHash, utxo.TxPos, lockingScript.String(), utxo.Value, unlocker)
		if err != nil {
			return fmt.Errorf("failed to add input: %w", err)
		}
		totalInput += utxo.Value
	}

	// Add OP_RETURN output
	if len(parts) == 1 {
		if err := tx.AddOpReturnOutput(parts[0]); err != nil {
			return fmt.Errorf("failed to add OP_RETURN output: %w", err)
		}
	} else {
		if err := tx.AddOpReturnPartsOutput(parts); err != nil {
			return fmt.Errorf("failed to add OP_RETURN output: %w", err)
		}
	}

	// Calculate fee and add change output
	estimatedSize := uint64(len(tx.Inputs)*inputSize + (len(tx.Outputs)+1)*outputSize + baseTxSize)
	fee := (estimatedSize * feePerKb) / 1000
	if fee < minFee {
		fee = minFee
	}

	if debug {
		log.Printf("Total input: %d sats, Estimated fee: %d sats", totalInput, fee)
	}

	change := totalInput - fee
	if change > dustLimit {
		changeLockingScript, err := p2pkh.Lock(sourceAddr)
		if err != nil {
			return fmt.Errorf("failed to create change locking script: %w", err)
		}
		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      change,
			LockingScript: changeLockingScript,
		})
		if debug {
			log.Printf("Change: %d sats", change)
		}
	} else if debug {
		log.Printf("Change (%d sats) below dust limit, adding to fee", change)
	}

	// Sign
	if err := tx.Sign(); err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	if debug {
		log.Printf("Transaction ID: %s", tx.TxID().String())
	}

	fmt.Println(tx.String())
	return nil
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

func getUnspentOutputs(ctx context.Context, addr string) ([]*UTXO, error) {
	network := "main"
	if testnet {
		network = "test"
	}

	url := fmt.Sprintf("https://api.whatsonchain.com/v1/bsv/%s/address/%s/unspent/all", network, addr)

	if debug {
		log.Printf("Fetching UTXOs from WhatsOnChain (%s)...", network)
	}

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to fetch UTXOs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WhatsOnChain API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response WOCUnspentAllResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse UTXOs: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("API error: %s", response.Error)
	}

	// Filter and deduplicate
	seen := make(map[string]bool)
	var utxos []*UTXO
	for _, u := range response.Result {
		if u.IsSpentInMempoolTx {
			continue
		}
		key := fmt.Sprintf("%s:%d", u.TxHash, u.TxPos)
		if seen[key] {
			continue
		}
		seen[key] = true
		utxos = append(utxos, &UTXO{
			TxHash: u.TxHash,
			TxPos:  uint32(u.TxPos),
			Value:  u.Value,
		})
	}

	return utxos, nil
}

func selectUTXOs(utxos []*UTXO, targetAmount, feeRate uint64) ([]*UTXO, error) {
	if len(utxos) == 0 {
		return nil, fmt.Errorf("no UTXOs available")
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
	return nil, fmt.Errorf("insufficient funds: have %d sats, need %d (amount: %d + fee: ~%d)",
		totalValue, targetAmount+estimatedFee, targetAmount, estimatedFee)
}

func calculateFee(numInputs, numOutputs int, feeRate uint64) uint64 {
	size := uint64(numInputs*inputSize + numOutputs*outputSize + baseTxSize)
	fee := (size * feeRate) / 1000
	if fee < minFee {
		fee = minFee
	}
	return fee
}

func init() {
	rootCmd.Flags().StringVarP(&wifFlag, "wif", "w", "", "WIF private key for signing (required)")
	rootCmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet")
	rootCmd.Flags().Uint64VarP(&feePerKb, "fee-per-kb", "f", 100, "Fee per kilobyte in satoshis")
	rootCmd.Flags().Uint64VarP(&dustLimit, "dust", "d", 1, "Dust limit in satoshis")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.MarkFlagRequired("wif") //nolint:errcheck
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
