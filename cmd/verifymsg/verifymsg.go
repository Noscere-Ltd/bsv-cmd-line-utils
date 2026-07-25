// Package main implements the verifymsg CLI tool, which verifies a Bitcoin
// Signed Message signature against a BSV address.
package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	bsm "github.com/bsv-blockchain/go-sdk/compat/bsm"
	"github.com/spf13/cobra"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
)

var (
	errAddressAndSignatureRequired = errors.New("--address and --signature are required")
	errNoMessageInput              = errors.New("no message provided (use --message or pipe via stdin)")
)

func newRootCmd() *cobra.Command {
	var (
		addressFlag   string
		signatureFlag string
		messageFlag   string
	)

	cmd := &cobra.Command{
		Use:   "verifymsg",
		Short: "Verify a Bitcoin Signed Message",
		Long:  "A command line tool that verifies a Bitcoin Signed Message signature against an address. Exits 0 if valid, 1 if invalid",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, addressFlag, signatureFlag, messageFlag)
		},
	}

	cmd.Flags().StringVarP(&addressFlag, "address", "a", "", "BSV address to verify against (required)")
	cmd.Flags().StringVarP(&signatureFlag, "signature", "s", "", "Base64-encoded signature (required)")
	cmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Message to verify")

	if err := cmd.MarkFlagRequired("address"); err != nil {
		panic(err)
	}

	if err := cmd.MarkFlagRequired("signature"); err != nil {
		panic(err)
	}

	return cmd
}

func run(cmd *cobra.Command, addressFlag, signatureFlag, messageFlag string) error {
	if addressFlag == "" || signatureFlag == "" {
		if err := cmd.Help(); err != nil {
			return err
		}

		return errAddressAndSignatureRequired
	}

	message, err := getMessage(messageFlag)
	if err != nil {
		return err
	}

	if message == "" {
		if helpErr := cmd.Help(); helpErr != nil {
			return helpErr
		}

		return errNoMessageInput
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signatureFlag)
	if err != nil {
		return fmt.Errorf("invalid base64 signature: %w", err)
	}

	err = bsm.VerifyMessage(addressFlag, sigBytes, []byte(message))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid: %v\n", err)

		os.Exit(1)
	}

	_, _ = fmt.Fprintln(os.Stdout, "Valid")

	return nil
}

func getMessage(messageFlag string) (string, error) {
	if messageFlag != "" {
		return messageFlag, nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadTextFromReader(os.Stdin)
	}

	return "", nil
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}
