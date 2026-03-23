package main

import (
	"encoding/base64"
	"fmt"
	"os"

	bsm "github.com/bsv-blockchain/go-sdk/compat/bsm"
	"github.com/spf13/cobra"

	"github.com/n0sc/bsv-cmd-line-utils/internal/cli"
)

var (
	addressFlag   string
	signatureFlag string
	messageFlag   string
)

var rootCmd = &cobra.Command{
	Use:   "verifymsg",
	Short: "Verify a Bitcoin Signed Message",
	Long:  "A command line tool that verifies a Bitcoin Signed Message signature against an address. Exits 0 if valid, 1 if invalid",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func run(cmd *cobra.Command) error {
	if addressFlag == "" || signatureFlag == "" {
		_ = cmd.Help()
		return fmt.Errorf("--address and --signature are required")
	}

	message, err := getMessage(cmd)
	if err != nil {
		return err
	}

	if message == "" {
		_ = cmd.Help()
		return fmt.Errorf("no message provided (use --message or pipe via stdin)")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureFlag)
	if err != nil {
		return fmt.Errorf("invalid base64 signature: %w", err)
	}

	err = bsm.VerifyMessage(addressFlag, sigBytes, []byte(message))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "Valid")
	return nil
}

func getMessage(_ *cobra.Command) (string, error) {
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
	rootCmd.Flags().StringVarP(&addressFlag, "address", "a", "", "BSV address to verify against (required)")
	rootCmd.Flags().StringVarP(&signatureFlag, "signature", "s", "", "Base64-encoded signature (required)")
	rootCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Message to verify")
	if err := rootCmd.MarkFlagRequired("address"); err != nil {
		panic(err)
	}
	if err := rootCmd.MarkFlagRequired("signature"); err != nil {
		panic(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
