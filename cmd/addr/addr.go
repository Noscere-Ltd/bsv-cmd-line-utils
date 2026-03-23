package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	base58 "github.com/bsv-blockchain/go-sdk/compat/base58"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/spf13/cobra"

	"github.com/n0sc/bsv-cmd-line-utils/internal/cli"
)

const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorWhite = "\033[37m"
	colorDim   = "\033[2m"
)

var (
	jsonFlag bool
	noColor  bool
)

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

var rootCmd = &cobra.Command{
	Use:   "addr [address_or_pubkey]",
	Short: "Validate a BSV address or derive addresses from a public key",
	Long:  "A command line tool that validates BSV addresses (showing network and hash160) or derives mainnet/testnet addresses from a public key hex",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, args)
	},
}

func run(cmd *cobra.Command, args []string) error {
	input, err := getInput(cmd, args)
	if err != nil {
		return err
	}

	if input == "" {
		_ = cmd.Help()
		return fmt.Errorf("no address or public key provided")
	}

	// Auto-detect: 66 or 130 hex chars = public key, otherwise treat as address
	if isPubKeyHex(input) {
		return deriveModeRun(input)
	}
	return validateModeRun(input)
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

func validateModeRun(addr string) error {
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

		// Detect network from version byte
		decoded, err := base58.Decode(addr)
		if err == nil && len(decoded) > 0 {
			switch decoded[0] {
			case 0x00:
				result.Network = "mainnet"
			case 0x6f:
				result.Network = "testnet"
			default:
				result.Network = fmt.Sprintf("unknown (0x%02x)", decoded[0])
			}
		}
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printValidateHuman(&result)
	return nil
}

func deriveModeRun(pubKeyHex string) error {
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

	printDeriveHuman(&result)
	return nil
}

func c(color, text string) string {
	if noColor {
		return text
	}
	return color + text + colorReset
}

func printValidateHuman(result *validateResult) {
	fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Address:"), c(colorGreen, result.Address))
	validStr := "yes"
	if !result.Valid {
		validStr = "no"
	}
	fmt.Fprintf(os.Stdout, "%s   %s\n", c(colorDim, "Valid:"), c(colorGreen, validStr))
	if result.Valid {
		fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Network:"), c(colorGreen, result.Network))
		fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Hash160:"), c(colorGreen, result.Hash160))
	}
}

func printDeriveHuman(result *deriveResult) {
	fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Public Key:"), c(colorGreen, result.PublicKey))
	fmt.Fprintf(os.Stdout, "%s  %s\n", c(colorDim, "Mainnet:"), c(colorGreen, result.Mainnet))
	fmt.Fprintf(os.Stdout, "%s  %s\n", c(colorDim, "Testnet:"), c(colorGreen, result.Testnet))
}

func init() {
	rootCmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON format")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
