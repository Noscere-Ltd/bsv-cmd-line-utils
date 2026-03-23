// Package main implements a Bitcoin SV key pair generator.
//
// This tool generates BSV private keys with their corresponding public keys and addresses.
// It outputs keys in WIF (Wallet Import Format) and hex formats, supporting both
// mainnet and testnet networks.
//
// Features:
//   - Mainnet/testnet support via --testnet flag
//   - Compressed/uncompressed key format via --uncompressed flag
//   - Generate multiple key pairs via --count flag
//   - JSON output format via --json flag
//   - Cryptographically secure key generation using the BSV SDK
//
// Usage:
//
//	keygen                          # Generate single mainnet key pair
//	keygen -t                       # Generate testnet key pair
//	keygen -u                       # Generate with uncompressed public key
//	keygen -c 5                     # Generate 5 key pairs
//	keygen -j                       # Output in JSON format
//	keygen -t -c 3 -j               # Generate 3 testnet keys in JSON
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/spf13/cobra"
)

// Network prefixes for WIF encoding
const (
	mainnetWIFPrefix = 0x80
	testnetWIFPrefix = 0xef
)

// Command-line flags
var (
	testnet      bool // Use testnet instead of mainnet
	uncompressed bool // Generate uncompressed keys
	count        int  // Number of key pairs to generate
	jsonOutput   bool // Output in JSON format
)

// KeyPair holds the generated key information.
type KeyPair struct {
	PrivateKey string `json:"privateKey"` // Private key in hex format
	PublicKey  string `json:"publicKey"`  // Public key in hex format
	WIF        string `json:"wif"`        // Private key in WIF format
	Address    string `json:"address"`    // P2PKH address
	Network    string `json:"network"`    // Network name (mainnet/testnet)
	Compressed bool   `json:"compressed"` // Whether the key is compressed
}

// rootCmd is the main cobra command for the keygen tool.
var rootCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate BSV key pairs",
	Long: `A command line tool that generates Bitcoin SV private keys with their
corresponding public keys and addresses. Keys are generated using
cryptographically secure random number generation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

// run handles the main execution flow.
func run() error {
	// Validate count
	if count < 1 || count > 100 {
		return fmt.Errorf("count must be between 1 and 100")
	}

	// Generate key pairs
	keyPairs := make([]KeyPair, 0, count)
	for i := 0; i < count; i++ {
		kp, err := generateKeyPair()
		if err != nil {
			return fmt.Errorf("generating key pair: %w", err)
		}
		keyPairs = append(keyPairs, kp)
	}

	// Output results
	if jsonOutput {
		return outputJSON(keyPairs)
	}
	return outputText(keyPairs)
}

// generateKeyPair creates a new BSV key pair.
func generateKeyPair() (KeyPair, error) {
	// Generate new private key
	privKey, err := ec.NewPrivateKey()
	if err != nil {
		return KeyPair{}, fmt.Errorf("creating private key: %w", err)
	}

	// Get public key
	pubKey := privKey.PubKey()

	// Determine WIF prefix based on network
	wifPrefix := byte(mainnetWIFPrefix)
	if testnet {
		wifPrefix = testnetWIFPrefix
	}

	// Get WIF with appropriate prefix
	wif := privKey.WifPrefix(wifPrefix)

	// Handle uncompressed WIF (remove compression flag byte before checksum)
	if uncompressed {
		wif = generateUncompressedWIF(privKey.Serialize(), wifPrefix)
	}

	// Get public key hex (compressed or uncompressed)
	var pubKeyHex string
	if uncompressed {
		pubKeyHex = hex.EncodeToString(pubKey.Uncompressed())
	} else {
		pubKeyHex = hex.EncodeToString(pubKey.Compressed())
	}

	// Generate address
	mainnet := !testnet
	address, err := script.NewAddressFromPublicKeyWithCompression(pubKey, mainnet, !uncompressed)
	if err != nil {
		return KeyPair{}, fmt.Errorf("creating address: %w", err)
	}

	// Determine network name
	network := "mainnet"
	if testnet {
		network = "testnet"
	}

	return KeyPair{
		PrivateKey: privKey.Hex(),
		PublicKey:  pubKeyHex,
		WIF:        wif,
		Address:    address.AddressString,
		Network:    network,
		Compressed: !uncompressed,
	}, nil
}

// generateUncompressedWIF creates a WIF string for an uncompressed key.
// Uncompressed WIF does not include the 0x01 compression flag byte.
func generateUncompressedWIF(privKeyBytes []byte, prefix byte) string {
	// WIF format: prefix (1 byte) + private key (32 bytes) + checksum (4 bytes)
	// WIF format for uncompressed keys: no compression flag byte
	payload := make([]byte, 1+len(privKeyBytes))
	payload[0] = prefix
	copy(payload[1:], privKeyBytes)

	// Add checksum
	return script.Base58EncodeMissingChecksum(payload)
}

// outputJSON prints key pairs in JSON format.
func outputJSON(keyPairs []KeyPair) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(keyPairs) //nolint:gosec // G117: intentionally marshaling private keys in key generator
}

// outputText prints key pairs in human-readable format.
func outputText(keyPairs []KeyPair) error {
	fmt.Fprint(os.Stdout, "\n=== BSV Key Generator ===\n\n")

	for i, kp := range keyPairs {
		if count > 1 {
			fmt.Fprintf(os.Stdout, "Key #%d:\n", i+1)
		}
		fmt.Fprintf(os.Stdout, "Network: %s\n", kp.Network)
		fmt.Fprintf(os.Stdout, "Private Key (hex): %s\n", kp.PrivateKey)
		fmt.Fprintf(os.Stdout, "Public Key (hex): %s\n", kp.PublicKey)
		fmt.Fprintf(os.Stdout, "WIF: %s\n", kp.WIF)
		fmt.Fprintf(os.Stdout, "Address: %s\n", kp.Address)
		fmt.Fprintf(os.Stdout, "Compressed: %t\n", kp.Compressed)

		if i < len(keyPairs)-1 {
			fmt.Fprintln(os.Stdout, "---")
		}
	}

	fmt.Fprintln(os.Stdout, "\nKeep your private keys secure!")
	return nil
}

// init initializes the cobra command flags.
func init() {
	rootCmd.Flags().BoolVarP(&testnet, "testnet", "t", false, "Generate testnet keys (default: mainnet)")
	rootCmd.Flags().BoolVarP(&uncompressed, "uncompressed", "u", false, "Generate uncompressed keys (default: compressed)")
	rootCmd.Flags().IntVarP(&count, "count", "c", 1, "Number of key pairs to generate (1-100)")
	rootCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
}

// main is the entry point for the keygen command.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
