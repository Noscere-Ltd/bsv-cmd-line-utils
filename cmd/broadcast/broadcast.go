// Package main implements a Bitcoin SV transaction broadcaster using ARC (BSV Transaction Processing).
//
// This tool broadcasts raw Bitcoin transactions to the BSV network via ARC endpoints
// and optionally monitors their status until they reach a final state (MINED, REJECTED, etc.).
//
// Features:
//   - Config-based mainnet/testnet endpoint management via config.yaml
//   - Real-time transaction status monitoring with customizable polling
//   - Support for stdin or command-line input
//   - Automatic transaction lifecycle tracking
//
// Usage:
//
//	echo "010000..." | broadcast              # Broadcast from stdin
//	broadcast -r "010000..."                  # Broadcast using flag
//	broadcast -t -m                           # Testnet with monitoring
//	broadcast -m -p 10                        # Monitor with 10s poll rate
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/arc"
	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/config"
	"github.com/spf13/cobra"
)

var (
	errNoTransactionProvided = errors.New("no transaction provided")
	errInvalidHexInput       = errors.New("input is not a valid hex string")
)

// newRootCmd builds the cobra command for the broadcast tool.
func newRootCmd() *cobra.Command {
	var (
		testnet  bool   // Use testnet instead of mainnet
		raw      string // Raw transaction hex provided via flag
		monitor  bool   // Enable transaction status monitoring
		pollRate int    // Polling interval in seconds for monitoring
	)

	cmd := &cobra.Command{
		Use:   "broadcast",
		Short: "A simple transaction broadcaster",
		Long:  "A command line tool that broadcasts bitcoin transactions from stdin",
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(testnet, raw, monitor, pollRate)
		},
	}

	cmd.Flags().StringVarP(&raw, "raw", "r", "", "Raw transaction hex to broadcast")
	cmd.Flags().BoolVarP(&monitor, "monitor", "m", false, "Monitor transaction status until final state")
	cmd.Flags().IntVarP(&pollRate, "poll-rate", "p", 5, "Polling rate in seconds for monitoring (default: 5)")
	cmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet configuration from config.yaml")

	return cmd
}

// run handles the main execution flow:
// 1. Loads configuration from config.yaml
// 2. Reads transaction hex from flag or stdin
// 3. Validates the hex string
// 4. Broadcasts the transaction to ARC
func run(testnet bool, raw string, monitor bool, pollRate int) error {
	// Load configuration from config.yaml
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Validate config
	if validateErr := cfg.Validate(testnet); validateErr != nil {
		return validateErr
	}

	// Get transaction from raw flag or stdin
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

	_, _ = fmt.Fprintf(os.Stdout, "Transaction hex: %s\n", txString)

	// Broadcast transaction using ARC
	return broadcastTransaction(cfg, txString, testnet, monitor, pollRate)
}

// getTransactionHex reads transaction hex from flag or stdin.
func getTransactionHex(raw string) (string, error) {
	if raw != "" {
		return raw, nil
	}

	return cli.ReadHexFromReader(os.Stdin)
}

// broadcastTransaction sends a raw transaction to the ARC network.
// It selects the appropriate endpoint (mainnet/testnet) based on the --testnet flag,
// creates an ARC client, broadcasts the transaction, and displays the result.
// If --monitor flag is set, it will continuously poll the transaction status.
func broadcastTransaction(cfg *config.Config, rawTx string, testnet, monitor bool, pollRate int) error {
	arcConfig := cfg.GetARCConfig(testnet)

	if testnet {
		_, _ = fmt.Fprintln(os.Stdout, "Using testnet configuration")
	} else {
		_, _ = fmt.Fprintln(os.Stdout, "Using mainnet configuration")
	}

	// Create ARC client
	client := arc.NewARCClient(arcConfig.URL, arcConfig.APIKey)

	_, _ = fmt.Fprintln(os.Stdout, "Broadcasting transaction to ARC...")

	// Broadcast the transaction
	resp, err := client.BroadcastTransaction(rawTx)
	if err != nil {
		return fmt.Errorf("broadcasting transaction: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✓ Transaction broadcast successful!\n")
	_, _ = fmt.Fprintf(os.Stdout, "  TxID: %s\n", resp.TxID)
	_, _ = fmt.Fprintf(os.Stdout, "  Status: %s\n", resp.TxStatus)
	_, _ = fmt.Fprintf(os.Stdout, "  Description: %s\n", arc.GetStatusDescription(resp.TxStatus))

	if resp.ExtraInfo != "" {
		_, _ = fmt.Fprintf(os.Stdout, "  Info: %s\n", resp.ExtraInfo)
	}

	// Monitor transaction status if requested
	if monitor {
		monitorTransaction(client, resp.TxID, pollRate)
	}

	return nil
}

// monitorTransaction continuously polls the transaction status until it reaches a final state.
// Final states are: MINED, REJECTED, or DOUBLE_SPEND_ATTEMPTED.
// The polling interval is controlled by the --poll-rate flag (default: 5 seconds).
// Displays timestamped status updates and block information if available.
func monitorTransaction(client *arc.Client, txid string, pollRate int) {
	_, _ = fmt.Fprintf(os.Stdout, "\nMonitoring transaction status (polling every %d seconds)...\n", pollRate)
	_, _ = fmt.Fprintln(os.Stdout, "Press Ctrl+C to stop monitoring")
	_, _ = fmt.Fprintln(os.Stdout)

	ticker := time.NewTicker(time.Duration(pollRate) * time.Second)
	defer ticker.Stop()

	for {
		status, err := client.GetTransactionStatus(txid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting transaction status: %v\n", err)
			<-ticker.C

			continue
		}

		timestamp := time.Now().Format("15:04:05")
		_, _ = fmt.Fprintf(os.Stdout, "[%s] Status: %s - %s\n", timestamp, status.TxStatus, arc.GetStatusDescription(status.TxStatus))

		if status.BlockHash != "" {
			_, _ = fmt.Fprintf(os.Stdout, "         Block Hash: %s\n", status.BlockHash)
			_, _ = fmt.Fprintf(os.Stdout, "         Block Height: %d\n", status.BlockHeight)
		}

		// Stop monitoring if transaction reached final state
		if arc.IsTransactionFinal(status.TxStatus) {
			_, _ = fmt.Fprintf(os.Stdout, "\n✓ Transaction reached final state: %s\n", status.TxStatus)
			break
		}

		<-ticker.C
	}
}

// main is the entry point for the broadcast command.
// It executes the cobra root command which handles flag parsing and command execution.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
