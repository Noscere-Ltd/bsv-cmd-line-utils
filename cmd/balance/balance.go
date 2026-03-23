package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/mrz1836/go-whatsonchain"
	"github.com/spf13/cobra"

	"github.com/n0sc/bsv-cmd-line-utils/internal/cli"
)

const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorDim   = "\033[2m"
)

var (
	testnet  bool
	jsonFlag bool
	utxos    bool
	noColor  bool
)

type balanceResult struct {
	Address     string       `json:"address"`
	Confirmed   int64        `json:"confirmed"`
	Unconfirmed int64        `json:"unconfirmed"`
	Total       int64        `json:"total"`
	BSV         float64      `json:"bsv"`
	UTXOs       []utxoRecord `json:"utxos,omitempty"`
}

type utxoRecord struct {
	TxHash string `json:"txHash"`
	TxPos  int64  `json:"txPos"`
	Value  int64  `json:"value"`
	Height int64  `json:"height"`
}

var rootCmd = &cobra.Command{
	Use:   "balance [address_or_wif]",
	Short: "Check the balance of a BSV address",
	Long:  "A command line tool that checks the balance of a BSV address via WhatsOnChain. Accepts an address or WIF as input",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, args)
	},
}

func run(cmd *cobra.Command, args []string) error {
	input, err := getInput(cmd, args)
	if err != nil {
		return err
	}

	if input == "" {
		cmd.Help() //nolint:errcheck
		return fmt.Errorf("no address or WIF provided")
	}

	// Auto-detect: try WIF first, fall back to address
	addr, err := resolveAddress(input)
	if err != nil {
		return err
	}

	ctx := context.Background()

	var client whatsonchain.ClientInterface
	if testnet {
		client, err = whatsonchain.NewClient(ctx, whatsonchain.WithNetwork(whatsonchain.NetworkTest))
	} else {
		client, err = whatsonchain.NewClient(ctx, whatsonchain.WithNetwork(whatsonchain.NetworkMain))
	}
	if err != nil {
		return fmt.Errorf("creating WhatsOnChain client: %w", err)
	}

	bal, err := client.AddressBalance(ctx, addr)
	if err != nil {
		return fmt.Errorf("fetching balance: %w", err)
	}

	total := bal.Confirmed + bal.Unconfirmed
	result := balanceResult{
		Address:     addr,
		Confirmed:   bal.Confirmed,
		Unconfirmed: bal.Unconfirmed,
		Total:       total,
		BSV:         float64(total) / 1e8,
	}

	if utxos {
		history, err := client.AddressUnspentTransactions(ctx, addr)
		if err != nil {
			return fmt.Errorf("fetching UTXOs: %w", err)
		}
		for _, h := range history {
			result.UTXOs = append(result.UTXOs, utxoRecord{
				TxHash: h.TxHash,
				TxPos:  h.TxPos,
				Value:  h.Value,
				Height: h.Height,
			})
		}
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printHuman(&result)
	return nil
}

func getInput(cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

func resolveAddress(input string) (string, error) {
	// Try as WIF first
	privKey, err := ec.PrivateKeyFromWif(input)
	if err == nil {
		addr, err := script.NewAddressFromPublicKey(privKey.PubKey(), !testnet)
		if err != nil {
			return "", fmt.Errorf("deriving address from WIF: %w", err)
		}
		return addr.AddressString, nil
	}

	// Treat as address
	return input, nil
}

func c(color, text string) string {
	if noColor {
		return text
	}
	return color + text + colorReset
}

func printHuman(result *balanceResult) {
	fmt.Printf("%s %s\n", c(colorDim, "Address:"), c(colorGreen, result.Address))
	fmt.Printf("%s %s\n", c(colorDim, "Confirmed:"), c(colorGreen, fmt.Sprintf("%d sats", result.Confirmed)))
	fmt.Printf("%s %s\n", c(colorDim, "Unconfirmed:"), c(colorGreen, fmt.Sprintf("%d sats", result.Unconfirmed)))
	fmt.Printf("%s %s\n", c(colorDim, "Total:"), c(colorGreen, fmt.Sprintf("%d sats (%.8f BSV)", result.Total, result.BSV)))

	if len(result.UTXOs) > 0 {
		fmt.Printf("\n%s\n", c(colorDim, "UTXOs:"))
		for _, u := range result.UTXOs {
			fmt.Printf("  %s:%d  %s sats  (height: %d)\n",
				c(colorGreen, u.TxHash), u.TxPos,
				c(colorGreen, fmt.Sprintf("%d", u.Value)), u.Height)
		}
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet")
	rootCmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON format")
	rootCmd.Flags().BoolVarP(&utxos, "utxos", "u", false, "Show individual UTXOs")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
