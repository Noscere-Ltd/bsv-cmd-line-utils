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
	"fmt"
	"os"
	"time"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/arc"
	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/config"
	"github.com/spf13/cobra"
)

// Command-line flags
var (
	txid     string // Transaction ID provided via flag
	testnet  bool   // Use testnet instead of mainnet
	monitor  bool   // Enable transaction status monitoring
	pollRate int    // Polling interval in seconds for monitoring
)

// rootCmd is the main cobra command for the txstatus tool.
var rootCmd = &cobra.Command{
	Use:   "txstatus [txid]",
	Short: "Check transaction status",
	Long:  "A command line tool that checks transaction status on ARC. Accepts txid as argument or from stdin",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		transactionID, err := getTransactionID(cmd, args)
		if err != nil {
			return err
		}

		if transactionID == "" {
			cmd.Help()
			return fmt.Errorf("no txid provided")
		}

		// Validate it's a hex string
		if !cli.IsValidHex(transactionID) {
			return fmt.Errorf("txid is not a valid hex string: %s", transactionID)
		}

		return checkTransactionStatus(transactionID)
	},
}

// getTransactionID retrieves the transaction ID from argument, flag, or stdin.
func getTransactionID(cmd *cobra.Command, args []string) (string, error) {
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
func checkTransactionStatus(txid string) error {
	// Load configuration from config.yaml
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Validate config
	if err := cfg.Validate(testnet); err != nil {
		return err
	}

	arcConfig := cfg.GetARCConfig(testnet)

	if testnet {
		fmt.Println("Using testnet configuration")
	} else {
		fmt.Println("Using mainnet configuration")
	}

	// Create ARC client
	client := arc.NewARCClient(arcConfig.URL, arcConfig.APIKey)

	if monitor {
		// Continuous monitoring
		return monitorTransaction(client, txid)
	}

	// Single status check
	return getStatus(client, txid)
}

// getStatus performs a single transaction status check.
func getStatus(client *arc.ARCClient, txid string) error {
	fmt.Printf("Checking status for transaction: %s\n\n", txid)

	status, err := client.GetTransactionStatus(txid)
	if err != nil {
		return fmt.Errorf("getting transaction status: %w", err)
	}

	fmt.Printf("Status: %s\n", status.TxStatus)
	fmt.Printf("Description: %s\n", arc.GetStatusDescription(status.TxStatus))

	if status.ExtraInfo != "" {
		fmt.Printf("Info: %s\n", status.ExtraInfo)
	}

	if status.Timestamp != "" {
		fmt.Printf("Timestamp: %s\n", status.Timestamp)
	}

	if status.BlockHash != "" {
		fmt.Printf("Block Hash: %s\n", status.BlockHash)
		fmt.Printf("Block Height: %d\n", status.BlockHeight)
	}

	if arc.IsTransactionFinal(status.TxStatus) {
		fmt.Printf("\n✓ Transaction is in final state\n")
	} else {
		fmt.Printf("\n⏳ Transaction is still pending (use --monitor to watch for changes)\n")
	}

	return nil
}

// monitorTransaction continuously polls the transaction status until it reaches a final state.
func monitorTransaction(client *arc.ARCClient, txid string) error {
	fmt.Printf("Monitoring transaction: %s\n", txid)
	fmt.Printf("Polling every %d seconds...\n", pollRate)
	fmt.Println("Press Ctrl+C to stop monitoring")
	fmt.Println()

	// Do initial check immediately
	status, err := client.GetTransactionStatus(txid)
	if err != nil {
		return fmt.Errorf("getting transaction status: %w", err)
	}

	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] Status: %s - %s\n", timestamp, status.TxStatus, arc.GetStatusDescription(status.TxStatus))

	if status.BlockHash != "" {
		fmt.Printf("         Block Hash: %s\n", status.BlockHash)
		fmt.Printf("         Block Height: %d\n", status.BlockHeight)
	}

	// If already final, exit
	if arc.IsTransactionFinal(status.TxStatus) {
		fmt.Printf("\n✓ Transaction is already in final state: %s\n", status.TxStatus)
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
		fmt.Printf("[%s] Status: %s - %s\n", timestamp, status.TxStatus, arc.GetStatusDescription(status.TxStatus))

		if status.BlockHash != "" {
			fmt.Printf("         Block Hash: %s\n", status.BlockHash)
			fmt.Printf("         Block Height: %d\n", status.BlockHeight)
		}

		// Stop monitoring if transaction reached final state
		if arc.IsTransactionFinal(status.TxStatus) {
			fmt.Printf("\n✓ Transaction reached final state: %s\n", status.TxStatus)
			break
		}
	}

	return nil
}

// init initializes the cobra command flags.
func init() {
	rootCmd.Flags().StringVarP(&txid, "txid", "i", "", "Transaction ID to check")
	rootCmd.Flags().BoolVarP(&monitor, "monitor", "m", false, "Monitor transaction status until final state")
	rootCmd.Flags().IntVarP(&pollRate, "poll-rate", "p", 5, "Polling rate in seconds for monitoring (default: 5)")
	rootCmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet configuration from config.yaml")
}

// main is the entry point for the txstatus command.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
