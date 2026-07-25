// Package main implements the decodescript CLI tool for decoding Bitcoin scripts to human-readable ASM.
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/spf13/cobra"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
)

const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorWhite = "\033[37m"
	colorDim   = "\033[2m"
)

var (
	errNoScriptHexProvided = errors.New("no script hex provided")
	errInvalidHexString    = errors.New("invalid hex string")
)

type decodeResult struct {
	ASM     string `json:"asm"`
	Type    string `json:"type"`
	Size    int    `json:"size"`
	Address string `json:"address,omitempty"`
}

func newRootCmd() *cobra.Command {
	var (
		jsonFlag bool
		noColor  bool
	)

	cmd := &cobra.Command{
		Use:   "decodescript [hex]",
		Short: "Decode a Bitcoin script to human-readable ASM",
		Long:  "A command line tool that decodes a hex-encoded Bitcoin script into opcodes, detects script type, and extracts addresses",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args, jsonFlag, noColor)
		},
	}

	cmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON format")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	return cmd
}

func run(cmd *cobra.Command, args []string, jsonFlag, noColor bool) error {
	input, err := getInput(args)
	if err != nil {
		return err
	}

	if input == "" {
		if helpErr := cmd.Help(); helpErr != nil {
			return helpErr
		}

		return errNoScriptHexProvided
	}

	if !cli.IsValidHex(input) {
		return fmt.Errorf("%w: %s", errInvalidHexString, input)
	}

	result, err := decodeScript(input)
	if err != nil {
		return err
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(result)
	}

	return printHuman(result, noColor)
}

func decodeScript(input string) (*decodeResult, error) {
	scriptBytes, err := hex.DecodeString(input)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	s := script.Script(scriptBytes)

	result := &decodeResult{
		ASM:  s.ToASM(),
		Type: detectType(&s),
		Size: len(scriptBytes),
	}

	// Extract address for P2PKH scripts
	if s.IsP2PKH() && len(scriptBytes) == 25 {
		hash160 := scriptBytes[3:23]

		addr, err := script.NewAddressFromPublicKeyHash(hash160, true)
		if err == nil {
			result.Address = addr.AddressString
		}
	}

	return result, nil
}

func getInput(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

func detectType(s *script.Script) string {
	switch {
	case s.IsP2PKH():
		return "P2PKH"
	case s.IsP2PK():
		return "P2PK"
	case s.IsP2SH():
		return "P2SH"
	case s.IsMultiSigOut():
		return "MultiSig"
	case s.IsData():
		return "Data"
	default:
		return "Unknown"
	}
}

func c(noColor bool, color, text string) string {
	if noColor {
		return text
	}

	return color + text + colorReset
}

func printHuman(result *decodeResult, noColor bool) error {
	if _, err := fmt.Fprintf(os.Stdout, "%s %s\n", c(noColor, colorDim, "ASM:"), c(noColor, colorGreen, result.ASM)); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(os.Stdout, "%s %s\n", c(noColor, colorDim, "Type:"), c(noColor, colorGreen, result.Type)); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(os.Stdout, "%s %s\n", c(noColor, colorDim, "Size:"), c(noColor, colorGreen, fmt.Sprintf("%d bytes", result.Size))); err != nil {
		return err
	}

	if result.Address != "" {
		if _, err := fmt.Fprintf(os.Stdout, "%s %s\n", c(noColor, colorDim, "Address:"), c(noColor, colorGreen, result.Address)); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
