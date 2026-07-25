// Package main implements the addr CLI tool, which validates BSV addresses
// and derives mainnet/testnet addresses from a public key.
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	base58 "github.com/bsv-blockchain/go-sdk/compat/base58"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
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

var errNoAddressOrPubKey = errors.New("no address or public key provided")

// validateResult holds output when validating an address.
type validateResult struct {
	Address string `json:"address"`
	Valid   bool   `json:"valid"`
	Network string `json:"network"`
	Hash160 string `json:"hash160"`
}

// deriveResult holds output when deriving from a public key.
type deriveResult struct {
	PublicKey string `json:"publicKey"`
	Mainnet   string `json:"mainnet"`
	Testnet   string `json:"testnet"`
}

func newRootCmd() *cobra.Command {
	var (
		jsonFlag bool
		noColor  bool
	)

	cmd := &cobra.Command{
		Use:   "addr [address_or_pubkey]",
		Short: "Validate a BSV address or derive addresses from a public key",
		Long:  "A command line tool that validates BSV addresses (showing network and hash160) or derives mainnet/testnet addresses from a public key hex",
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
	input, err := getInput(cmd, args)
	if err != nil {
		return err
	}

	if input == "" {
		if err := cmd.Help(); err != nil {
			return err
		}

		return errNoAddressOrPubKey
	}

	// Auto-detect: 66 or 130 hex chars = public key, otherwise treat as address
	if isPubKeyHex(input) {
		return deriveModeRun(input, jsonFlag, noColor)
	}

	return validateModeRun(input, jsonFlag, noColor)
}

func getInput(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

func isPubKeyHex(s string) bool {
	if len(s) != 66 && len(s) != 130 {
		return false
	}

	return cli.IsValidHex(s)
}

// detectNetwork returns the network name encoded in a base58check address's version byte.
func detectNetwork(addr string) string {
	decoded, err := base58.Decode(addr)
	if err != nil || len(decoded) == 0 {
		return ""
	}

	switch decoded[0] {
	case 0x00:
		return "mainnet"
	case 0x6f:
		return "testnet"
	default:
		return fmt.Sprintf("unknown (0x%02x)", decoded[0])
	}
}

func validateModeRun(addr string, jsonFlag, noColor bool) error {
	valid, err := script.ValidateAddress(addr)
	if err != nil {
		return fmt.Errorf("address validation error: %w", err)
	}

	result := validateResult{
		Address: addr,
		Valid:   valid,
	}

	if valid {
		a, err := script.NewAddressFromString(addr)
		if err == nil {
			result.Hash160 = hex.EncodeToString(a.PublicKeyHash)
		}

		result.Network = detectNetwork(addr)
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(result)
	}

	printValidateHuman(&result, noColor)

	return nil
}

func deriveModeRun(pubKeyHex string, jsonFlag, noColor bool) error {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return fmt.Errorf("failed to decode public key hex: %w", err)
	}

	pubKey, err := ec.PublicKeyFromBytes(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	mainnetAddr, err := script.NewAddressFromPublicKey(pubKey, true)
	if err != nil {
		return fmt.Errorf("deriving mainnet address: %w", err)
	}

	testnetAddr, err := script.NewAddressFromPublicKey(pubKey, false)
	if err != nil {
		return fmt.Errorf("deriving testnet address: %w", err)
	}

	result := deriveResult{
		PublicKey: pubKeyHex,
		Mainnet:   mainnetAddr.AddressString,
		Testnet:   testnetAddr.AddressString,
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(result)
	}

	printDeriveHuman(&result, noColor)

	return nil
}

func c(color, text string, noColor bool) string {
	if noColor {
		return text
	}

	return color + text + colorReset
}

func printValidateHuman(result *validateResult, noColor bool) {
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Address:", noColor), c(colorGreen, result.Address, noColor))

	validStr := "yes"
	if !result.Valid {
		validStr = "no"
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s   %s\n", c(colorDim, "Valid:", noColor), c(colorGreen, validStr, noColor))

	if result.Valid {
		_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Network:", noColor), c(colorGreen, result.Network, noColor))
		_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Hash160:", noColor), c(colorGreen, result.Hash160, noColor))
	}
}

func printDeriveHuman(result *deriveResult, noColor bool) {
	_, _ = fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Public Key:", noColor), c(colorGreen, result.PublicKey, noColor))
	_, _ = fmt.Fprintf(os.Stdout, "%s  %s\n", c(colorDim, "Mainnet:", noColor), c(colorGreen, result.Mainnet, noColor))
	_, _ = fmt.Fprintf(os.Stdout, "%s  %s\n", c(colorDim, "Testnet:", noColor), c(colorGreen, result.Testnet, noColor))
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		os.Exit(1)
	}
}
