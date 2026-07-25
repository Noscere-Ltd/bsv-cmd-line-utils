// Package main implements a Bitcoin SV transaction status checker using ARC.
//
// This tool checks the status of transactions on the BSV network via ARC endpoints
// and optionally monitors their status until they reach a final state.
//
// Features:
//   - Config-based mainnet/testnet endpoint management via config.yaml
//   - Real-time transaction status monitoring with customizable polling
//   - Support for stdin, flag, or command-line argument input
//   - Automatic transaction lifecycle tracking
//
// Usage:
//
//	txstatus <txid>                          # Check by argument
//	txstatus -i <txid>                       # Check by flag
//	echo <txid> | txstatus                   # Check from stdin
//	txstatus <txid> -t                       # Check on testnet
//	txstatus <txid> -m                       # Monitor until final state
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
	errNoTxidProvided = errors.New("no txid provided")
	errInvalidTxidHex = errors.New("txid is not a valid hex string")
)

// newRootCmd builds the cobra command for the txstatus tool.
func newRootCmd() *cobra.Command {
	var (
		txid     string // Transaction ID provided via flag
		testnet  bool   // Use testnet instead of mainnet
		monitor  bool   // Enable transaction status monitoring
		pollRate int    // Polling interval in seconds for monitoring
	)

	cmd := &cobra.Command{
		Use:   "txstatus [txid]",
		Short: "Check transaction status",
		Long:  "A command line tool that checks transaction status on ARC. Accepts txid as argument or from stdin",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			transactionID, err := getTransactionID(txid, args)
			if err != nil {
				return err
			}

			if transactionID == "" {
				cmd.Help() //nolint:errcheck,gosec // help output error is not actionable here
				return errNoTxidProvided
			}

			// Validate it's a hex string
			if !cli.IsValidHex(transactionID) {
				return fmt.Errorf("%w: %s", errInvalidTxidHex, transactionID)
			}

			return checkTransactionStatus(transactionID, testnet, monitor, pollRate)
		},
	}

	cmd.Flags().StringVarP(&txid, "txid", "i", "", "Transaction ID to check")
	cmd.Flags().BoolVarP(&monitor, "monitor", "m", false, "Monitor transaction status until final state")
	cmd.Flags().IntVarP(&pollRate, "poll-rate", "p", 5, "Polling rate in seconds for monitoring (default: 5)")
	cmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet configuration from config.yaml")

	return cmd
}

// getTransactionID retrieves the transaction ID from argument, flag, or stdin.
func getTransactionID(txid string, args []string) (string, error) {
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

// checkTransactionStatus loads config and checks/monitors the transaction status.
func checkTransactionStatus(txid string, testnet, monitor bool, pollRate int) error {
	// Load configuration from config.yaml
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Validate config
	if validateErr := cfg.Validate(testnet); validateErr != nil {
		return validateErr
	}

	arcConfig := cfg.GetARCConfig(testnet)

	if testnet {
		_, _ = fmt.Fprintln(os.Stdout, "Using testnet configuration")
	} else {
		_, _ = fmt.Fprintln(os.Stdout, "Using mainnet configuration")
	}

	// Create ARC client
	client := arc.NewARCClient(arcConfig.URL, arcConfig.APIKey)

	if monitor {
		// Continuous monitoring
		return monitorTransaction(client, txid, pollRate)
	}

	// Single status check
	return getStatus(client, txid)
}

// getStatus performs a single transaction status check.
func getStatus(client *arc.Client, txid string) error {
	_, _ = fmt.Fprintf(os.Stdout, "Checking status for transaction: %s\n\n", txid)

	status, err := client.GetTransactionStatus(txid)
	if err != nil {
		return fmt.Errorf("getting transaction status: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Status: %s\n", status.TxStatus)
	_, _ = fmt.Fprintf(os.Stdout, "Description: %s\n", arc.GetStatusDescription(status.TxStatus))

	if status.ExtraInfo != "" {
		_, _ = fmt.Fprintf(os.Stdout, "Info: %s\n", status.ExtraInfo)
	}

	if status.Timestamp != "" {
		_, _ = fmt.Fprintf(os.Stdout, "Timestamp: %s\n", status.Timestamp)
	}

	if status.BlockHash != "" {
		_, _ = fmt.Fprintf(os.Stdout, "Block Hash: %s\n", status.BlockHash)
		_, _ = fmt.Fprintf(os.Stdout, "Block Height: %d\n", status.BlockHeight)
	}

	if arc.IsTransactionFinal(status.TxStatus) {
		_, _ = fmt.Fprintf(os.Stdout, "\n✓ Transaction is in final state\n")
	} else {
		_, _ = fmt.Fprintf(os.Stdout, "\n⏳ Transaction is still pending (use --monitor to watch for changes)\n")
	}

	return nil
}

// monitorTransaction continuously polls the transaction status until it reaches a final state.
func monitorTransaction(client *arc.Client, txid string, pollRate int) error {
	_, _ = fmt.Fprintf(os.Stdout, "Monitoring transaction: %s\n", txid)
	_, _ = fmt.Fprintf(os.Stdout, "Polling every %d seconds...\n", pollRate)
	_, _ = fmt.Fprintln(os.Stdout, "Press Ctrl+C to stop monitoring")
	_, _ = fmt.Fprintln(os.Stdout)

	// Do initial check immediately
	status, err := client.GetTransactionStatus(txid)
	if err != nil {
		return fmt.Errorf("getting transaction status: %w", err)
	}

	timestamp := time.Now().Format("15:04:05")
	_, _ = fmt.Fprintf(os.Stdout, "[%s] Status: %s - %s\n", timestamp, status.TxStatus, arc.GetStatusDescription(status.TxStatus))

	if status.BlockHash != "" {
		_, _ = fmt.Fprintf(os.Stdout, "         Block Hash: %s\n", status.BlockHash)
		_, _ = fmt.Fprintf(os.Stdout, "         Block Height: %d\n", status.BlockHeight)
	}

	// If already final, exit
	if arc.IsTransactionFinal(status.TxStatus) {
		_, _ = fmt.Fprintf(os.Stdout, "\n✓ Transaction is already in final state: %s\n", status.TxStatus)
		return nil
	}

	// Continue monitoring
	ticker := time.NewTicker(time.Duration(pollRate) * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		status, err := client.GetTransactionStatus(txid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting transaction status: %v\n", err)
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
	}

	return nil
}

// main is the entry point for the txstatus command.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
