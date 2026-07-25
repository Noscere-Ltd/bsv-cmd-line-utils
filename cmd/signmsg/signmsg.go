// Package main implements the signmsg CLI tool, which signs a message with a
// BSV private key using the Bitcoin Signed Message format.
package main

import (
	"errors"
	"fmt"
	"os"

	bsm "github.com/bsv-blockchain/go-sdk/compat/bsm"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/spf13/cobra"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
)

var (
	errWifRequired    = errors.New("--wif is required")
	errNoMessageInput = errors.New("no message provided (use --message or pipe via stdin)")
)

func newRootCmd() *cobra.Command {
	var (
		wifFlag     string
		messageFlag string
	)

	cmd := &cobra.Command{
		Use:   "signmsg",
		Short: "Sign a message with a BSV private key (Bitcoin Signed Message)",
		Long:  "A command line tool that signs a message using Bitcoin Signed Message format. Outputs base64 signature to stdout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, wifFlag, messageFlag)
		},
	}

	cmd.Flags().StringVarP(&wifFlag, "wif", "w", "", "WIF private key for signing (required)")
	cmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Message to sign")

	if err := cmd.MarkFlagRequired("wif"); err != nil {
		panic(err)
	}

	return cmd
}

func run(cmd *cobra.Command, wifFlag, messageFlag string) error {
	if wifFlag == "" {
		if err := cmd.Help(); err != nil {
			return err
		}

		return errWifRequired
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

	privKey, err := ec.PrivateKeyFromWif(wifFlag)
	if err != nil {
		return fmt.Errorf("failed to parse WIF: %w", err)
	}

	sig, err := bsm.SignMessageString(privKey, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	_, _ = fmt.Fprintln(os.Stdout, sig)

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
