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
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/n0sc/bsv-cmd-line-utils/internal/arc"
	"github.com/n0sc/bsv-cmd-line-utils/internal/cli"
	"github.com/n0sc/bsv-cmd-line-utils/internal/config"
)

// Command-line flags
var (
	testnet  bool   // Use testnet instead of mainnet
	raw      string // Raw transaction hex provided via flag
	monitor  bool   // Enable transaction status monitoring
	pollRate int    // Polling interval in seconds for monitoring
)

// rootCmd is the main cobra command for the broadcast tool.
var rootCmd = &cobra.Command{
	Use:   "broadcast",
	Short: "A simple transaction broadcaster",
	Long:  "A command line tool that broadcasts bitcoin transactions from stdin",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

// run handles the main execution flow:
// 1. Loads configuration from config.yaml
// 2. Reads transaction hex from flag or stdin
// 3. Validates the hex string
// 4. Broadcasts the transaction to ARC
func run() error {
	// Load configuration from config.yaml
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Validate config
	if err := cfg.Validate(testnet); err != nil {
		return err
	}

	// Get transaction from raw flag or stdin
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

	fmt.Printf("Transaction hex: %s\n", txString)

	// Broadcast transaction using ARC
	return broadcastTransaction(cfg, txString)
}

// getTransactionHex reads transaction hex from flag or stdin.
func getTransactionHex() (string, error) {
	if raw != "" {
		return raw, nil
	}
	return cli.ReadHexFromReader(os.Stdin)
}

// broadcastTransaction sends a raw transaction to the ARC network.
// It selects the appropriate endpoint (mainnet/testnet) based on the --testnet flag,
// creates an ARC client, broadcasts the transaction, and displays the result.
// If --monitor flag is set, it will continuously poll the transaction status.
func broadcastTransaction(cfg *config.Config, rawTx string) error {
	arcConfig := cfg.GetARCConfig(testnet)

	if testnet {
		fmt.Println("Using testnet configuration")
	} else {
		fmt.Println("Using mainnet configuration")
	}

	// Create ARC client
	client := arc.NewARCClient(arcConfig.URL, arcConfig.APIKey)

	fmt.Println("Broadcasting transaction to ARC...")

	// Broadcast the transaction
	resp, err := client.BroadcastTransaction(rawTx)
	if err != nil {
		return fmt.Errorf("broadcasting transaction: %w", err)
	}

	fmt.Printf("✓ Transaction broadcast successful!\n")
	fmt.Printf("  TxID: %s\n", resp.TxID)
	fmt.Printf("  Status: %s\n", resp.TxStatus)
	fmt.Printf("  Description: %s\n", arc.GetStatusDescription(resp.TxStatus))
	if resp.ExtraInfo != "" {
		fmt.Printf("  Info: %s\n", resp.ExtraInfo)
	}

	// Monitor transaction status if requested
	if monitor {
		monitorTransaction(client, resp.TxID)
	}

	return nil
}

// monitorTransaction continuously polls the transaction status until it reaches a final state.
// Final states are: MINED, REJECTED, or DOUBLE_SPEND_ATTEMPTED.
// The polling interval is controlled by the --poll-rate flag (default: 5 seconds).
// Displays timestamped status updates and block information if available.
func monitorTransaction(client *arc.ARCClient, txid string) {
	fmt.Printf("\nMonitoring transaction status (polling every %d seconds)...\n", pollRate)
	fmt.Println("Press Ctrl+C to stop monitoring")
	fmt.Println()

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

		<-ticker.C
	}
}

// init initializes the cobra command flags.
// This function is automatically called by Go before main() executes.
func init() {
	rootCmd.Flags().StringVarP(&raw, "raw", "r", "", "Raw transaction hex to broadcast")
	rootCmd.Flags().BoolVarP(&monitor, "monitor", "m", false, "Monitor transaction status until final state")
	rootCmd.Flags().IntVarP(&pollRate, "poll-rate", "p", 5, "Polling rate in seconds for monitoring (default: 5)")
	rootCmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Use testnet configuration from config.yaml")
}

// main is the entry point for the broadcast command.
// It executes the cobra root command which handles flag parsing and command execution.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
