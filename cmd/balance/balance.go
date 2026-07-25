// Package main implements the balance CLI tool for checking a BSV address's balance.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/mrz1836/go-whatsonchain"
	"github.com/spf13/cobra"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
)

const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorDim   = "\033[2m"
)

var errNoAddressOrWIF = errors.New("no address or WIF provided")

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

func newRootCmd() *cobra.Command {
	var (
		testnet  bool
		jsonFlag bool
		utxos    bool
		noColor  bool
	)

	cmd := &cobra.Command{
		Use:   "balance [address_or_wif]",
		Short: "Check the balance of a BSV address",
		Long:  "A command line tool that checks the balance of a BSV address via WhatsOnChain. Accepts an address or WIF as input",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args, testnet, jsonFlag, utxos, noColor)
		},
	}

	cmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet")
	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON format")
	cmd.Flags().BoolVarP(&utxos, "utxos", "u", false, "Show individual UTXOs")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	return cmd
}

func run(cmd *cobra.Command, args []string, testnet, jsonFlag, utxos, noColor bool) error {
	input, err := getInput(cmd, args)
	if err != nil {
		return err
	}

	if input == "" {
		cmd.Help() //nolint:errcheck,gosec // help output error is not actionable here
		return errNoAddressOrWIF
	}

	// Auto-detect: try WIF first, fall back to address
	addr, err := resolveAddress(input, testnet)
	if err != nil {
		return err
	}

	ctx := context.Background()

	client, err := newWhatsOnChainClient(ctx, testnet)
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
		if err := attachUTXOs(ctx, client, addr, &result); err != nil {
			return err
		}
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(result)
	}

	printHuman(&result, noColor)

	return nil
}

func newWhatsOnChainClient(ctx context.Context, testnet bool) (whatsonchain.ClientInterface, error) {
	if testnet {
		return whatsonchain.NewClient(ctx, whatsonchain.WithNetwork(whatsonchain.NetworkTest))
	}

	return whatsonchain.NewClient(ctx, whatsonchain.WithNetwork(whatsonchain.NetworkMain))
}

func attachUTXOs(ctx context.Context, client whatsonchain.ClientInterface, addr string, result *balanceResult) error {
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

	return nil
}

func getInput(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

func resolveAddress(input string, testnet bool) (string, error) {
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

func c(color, text string, noColor bool) string {
	if noColor {
		return text
	}

	return color + text + colorReset
}

func printHuman(result *balanceResult, noColor bool) {
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Address:", noColor), c(colorGreen, result.Address, noColor))
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Confirmed:", noColor), c(colorGreen, fmt.Sprintf("%d sats", result.Confirmed), noColor))
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Unconfirmed:", noColor), c(colorGreen, fmt.Sprintf("%d sats", result.Unconfirmed), noColor))
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Total:", noColor), c(colorGreen, fmt.Sprintf("%d sats (%.8f BSV)", result.Total, result.BSV), noColor))

	if len(result.UTXOs) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\n%s\n", c(colorDim, "UTXOs:", noColor))

		for _, u := range result.UTXOs {
			_, _ = fmt.Fprintf(os.Stdout, "  %s:%d  %s sats  (height: %d)\n",
				c(colorGreen, u.TxHash, noColor), u.TxPos,
				c(colorGreen, fmt.Sprintf("%d", u.Value), noColor), u.Height)
		}
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
