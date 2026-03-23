package main

import (
	"fmt"
	"os"

	bsm "github.com/bsv-blockchain/go-sdk/compat/bsm"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/spf13/cobra"

	"github.com/n0sc/bsv-cmd-line-utils/internal/cli"
)

var (
	wifFlag     string
	messageFlag string
)

var rootCmd = &cobra.Command{
	Use:   "signmsg",
	Short: "Sign a message with a BSV private key (Bitcoin Signed Message)",
	Long:  "A command line tool that signs a message using Bitcoin Signed Message format. Outputs base64 signature to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func run(cmd *cobra.Command) error {
	if wifFlag == "" {
		cmd.Help() //nolint:errcheck
		return fmt.Errorf("--wif is required")
	}

	message, err := getMessage(cmd)
	if err != nil {
		return err
	}

	if message == "" {
		cmd.Help() //nolint:errcheck
		return fmt.Errorf("no message provided (use --message or pipe via stdin)")
	}

	privKey, err := ec.PrivateKeyFromWif(wifFlag)
	if err != nil {
		return fmt.Errorf("failed to parse WIF: %w", err)
	}

	sig, err := bsm.SignMessageString(privKey, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	fmt.Println(sig)
	return nil
}

func getMessage(cmd *cobra.Command) (string, error) {
	if messageFlag != "" {
		return messageFlag, nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadTextFromReader(os.Stdin)
	}

	return "", nil
}

func init() {
	rootCmd.Flags().StringVarP(&wifFlag, "wif", "w", "", "WIF private key for signing (required)")
	rootCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Message to sign")
	rootCmd.MarkFlagRequired("wif") //nolint:errcheck
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
