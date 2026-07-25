// Package main implements a Bitcoin SV transaction fetcher using the WhatsOnChain API.
//
// This tool retrieves raw transaction data (hex) from the WhatsOnChain blockchain explorer.
// It supports both mainnet and testnet, and accepts transaction IDs via command-line argument,
// flag, or stdin for flexible integration with other tools.
//
// Features:
//   - Mainnet/testnet support via --testnet flag
//   - Flexible input: argument, flag, or stdin
//   - Direct integration with WhatsOnChain API
//   - Easy chaining with other tools (e.g., prettytx)
//
// Usage:
//
//	getraw <txid>                    # Fetch by argument
//	getraw -i <txid>                 # Fetch by flag
//	echo <txid> | getraw             # Fetch from stdin
//	getraw <txid> -t                 # Fetch from testnet
//	getraw <txid> | prettytx         # Chain with prettytx
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
	"github.com/mrz1836/go-whatsonchain"
	"github.com/spf13/cobra"
)

var (
	errNoTxidProvided = errors.New("no txid provided")
	errInvalidTxidHex = errors.New("txid is not a valid hex string")
)

// newRootCmd builds the main cobra command for the getraw tool.
func newRootCmd() *cobra.Command {
	var (
		testnet bool   // Use testnet instead of mainnet
		txid    string // Transaction ID provided via flag
	)

	cmd := &cobra.Command{
		Use:   "getraw [txid]",
		Short: "Get raw transaction data",
		Long:  "A command line tool that retrieves raw transaction data from WhatsOnChain. Accepts txid as argument or from stdin",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			transactionID, err := getTransactionID(cmd, args, txid)
			if err != nil {
				return err
			}

			if transactionID == "" {
				if err := cmd.Help(); err != nil {
					return err
				}

				return errNoTxidProvided
			}

			// Validate it's a hex string
			if !cli.IsValidHex(transactionID) {
				return fmt.Errorf("%w: %s", errInvalidTxidHex, transactionID)
			}

			return getRawFromWhatsOnChain(transactionID, testnet)
		},
	}

	cmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet instead of mainnet")
	cmd.Flags().StringVarP(&txid, "txid", "i", "", "Transaction ID to retrieve")

	return cmd
}

// getTransactionID retrieves the transaction ID from argument, flag, or stdin.
func getTransactionID(_ *cobra.Command, args []string, txid string) (string, error) {
	// Get txid from command line argument if provided
	if len(args) > 0 {
		return args[0], nil
	}

	// Use flag value if provided
	if txid != "" {
		return txid, nil
	}

	// Check if stdin has data
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped to stdin
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

// getRawFromWhatsOnChain fetches raw transaction data from the WhatsOnChain API.
// It creates a client for the appropriate network (mainnet/testnet) based on the
// --testnet flag, queries the API for the transaction, and prints the raw hex to stdout.
//
// Logs the chain and network information to stderr.
// Outputs the raw transaction hex to stdout for easy piping to other tools.
func getRawFromWhatsOnChain(txid string, testnet bool) error {
	ctx := context.Background()

	var client whatsonchain.ClientInterface

	var err error

	// Create client based on testnet flag
	if testnet {
		client, err = whatsonchain.NewClient(ctx, whatsonchain.WithNetwork(whatsonchain.NetworkTest))
	} else {
		client, err = whatsonchain.NewClient(ctx, whatsonchain.WithNetwork(whatsonchain.NetworkMain))
	}

	if err != nil {
		return fmt.Errorf("creating WhatsOnChain client: %w", err)
	}

	log.Printf("Chain: %s, Network: %s\n", client.Chain(), client.Network())

	// Get raw transaction data
	rawTx, err := client.GetRawTransactionData(ctx, txid)
	if err != nil {
		return fmt.Errorf("getting raw transaction: %w", err)
	}

	// Print the raw transaction hex
	_, _ = fmt.Fprintln(os.Stdout, rawTx)

	return nil
}

// main is the entry point for the getraw command.
// It executes the cobra root command which handles flag parsing and command execution.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}
