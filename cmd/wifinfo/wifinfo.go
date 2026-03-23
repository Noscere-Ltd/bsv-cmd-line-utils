// Package main implements a Bitcoin SV WIF (Wallet Import Format) key inspector.
//
// This tool parses a WIF-encoded private key and displays the corresponding
// public keys, addresses, and WIF representations for both mainnet and testnet.
// It detects the original network and compression format of the input WIF.
//
// Features:
//   - Parses and validates WIF private keys
//   - Detects network (mainnet/testnet) and compression from input
//   - Displays compressed and uncompressed public keys
//   - Shows mainnet and testnet addresses (compressed and uncompressed)
//   - Shows mainnet and testnet WIF (compressed and uncompressed)
//   - JSON output support
//   - Flexible input: argument, flag, or stdin
//
// Usage:
//
//	wifinfo <wif>                    # Parse WIF from argument
//	wifinfo -w <wif>                 # Parse WIF from flag
//	echo <wif> | wifinfo             # Parse WIF from stdin
//	wifinfo -j <wif>                 # Output as JSON
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	base58 "github.com/bsv-blockchain/go-sdk/compat/base58"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	crypto "github.com/bsv-blockchain/go-sdk/primitives/hash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/spf13/cobra"

	"github.com/n0sc/bsv-cmd-line-utils/internal/cli"
)

// Network prefix bytes for WIF encoding
const (
	mainnetWIFPrefix byte = 0x80
	testnetWIFPrefix byte = 0xef
	compressMagic    byte = 0x01
	privateKeyLen         = 32
)

// ANSI color codes for terminal output styling
const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorWhite = "\033[37m"
	colorDim   = "\033[2m"
)

// Command-line flags
var (
	wif         string // WIF string provided via flag
	jsonFlag    bool   // Output in JSON format
	showUncompr bool   // Include uncompressed keys, WIFs, and addresses
	noColor     bool   // Disable colored output
)

// wifInput holds the parsed properties of the input WIF.
type wifInput struct {
	WIF        string `json:"wif"`
	Network    string `json:"network"`
	Compressed bool   `json:"compressed"`
}

// keyPair holds compressed and optionally uncompressed forms.
type keyPair struct {
	Compressed   string `json:"compressed"`
	Uncompressed string `json:"uncompressed,omitempty"`
}

// networkInfo holds WIF and address for a single network.
type networkInfo struct {
	WIF     keyPair `json:"wif"`
	Address keyPair `json:"address"`
}

// publicKeyInfo holds public key hex values.
type publicKeyInfo struct {
	Compressed   string `json:"compressed"`
	Uncompressed string `json:"uncompressed,omitempty"`
}

// wifInfoResult holds the complete output for a parsed WIF.
type wifInfoResult struct {
	Input     wifInput      `json:"input"`
	PublicKey publicKeyInfo `json:"public_key"`
	Mainnet   networkInfo   `json:"mainnet"`
	Testnet   networkInfo   `json:"testnet"`
}

// rootCmd is the main cobra command for the wifinfo tool.
var rootCmd = &cobra.Command{
	Use:   "wifinfo [wif]",
	Short: "Display mainnet and testnet details for a BSV private key in WIF format",
	Long:  "A command line tool that parses a WIF-encoded BSV private key and displays public keys, addresses, and WIF representations for both mainnet and testnet",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, args)
	},
}

// run handles the main execution flow.
func run(cmd *cobra.Command, args []string) error {
	wifString, err := getWIF(cmd, args)
	if err != nil {
		return err
	}

	if wifString == "" {
		_ = cmd.Help()
		return fmt.Errorf("no WIF provided")
	}

	result, err := getWIFInfo(wifString)
	if err != nil {
		return err
	}

	if jsonFlag {
		return printJSON(result)
	}

	printHuman(result)
	return nil
}

// getWIF retrieves the WIF string from argument, flag, or stdin.
func getWIF(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	if wif != "" {
		return wif, nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

// parseWIF decodes and validates a WIF string, returning the private key bytes,
// network, and compression flag.
func parseWIF(wifString string) (privKeyBytes []byte, isTestnet, isCompressed bool, err error) {
	decoded, err := base58.Decode(wifString)
	if err != nil {
		return nil, false, false, fmt.Errorf("invalid base58 encoding: %w", err)
	}

	decodedLen := len(decoded)

	// Validate length: 1 prefix + 32 privkey + 4 checksum = 37 (uncompressed)
	//                   1 prefix + 32 privkey + 1 compress + 4 checksum = 38 (compressed)
	switch decodedLen {
	case 1 + privateKeyLen + 1 + 4:
		if decoded[33] != compressMagic {
			return nil, false, false, fmt.Errorf("invalid compression flag: 0x%02x", decoded[33])
		}
		isCompressed = true
	case 1 + privateKeyLen + 4:
		isCompressed = false
	default:
		return nil, false, false, fmt.Errorf("invalid WIF length: %d bytes", decodedLen)
	}

	// Detect network
	switch decoded[0] {
	case mainnetWIFPrefix:
		isTestnet = false
	case testnetWIFPrefix:
		isTestnet = true
	default:
		return nil, false, false, fmt.Errorf("unknown network prefix: 0x%02x", decoded[0])
	}

	// Validate checksum
	var payload []byte
	if isCompressed {
		payload = decoded[:1+privateKeyLen+1]
	} else {
		payload = decoded[:1+privateKeyLen]
	}
	expectedChecksum := crypto.Sha256d(payload)[:4]
	actualChecksum := decoded[decodedLen-4:]
	if !bytes.Equal(expectedChecksum, actualChecksum) {
		return nil, false, false, fmt.Errorf("invalid WIF checksum")
	}

	privKeyBytes = decoded[1 : 1+privateKeyLen]
	return privKeyBytes, isTestnet, isCompressed, nil
}

// encodeWIF generates a WIF string from a private key with the given network and compression.
func encodeWIF(privKeyBytes []byte, isTestnet, isCompressed bool) string {
	prefix := mainnetWIFPrefix
	if isTestnet {
		prefix = testnetWIFPrefix
	}

	size := 1 + privateKeyLen + 4
	if isCompressed {
		size++
	}

	buf := make([]byte, 0, size)
	buf = append(buf, prefix)
	buf = append(buf, privKeyBytes...)
	if isCompressed {
		buf = append(buf, compressMagic)
	}

	checksum := crypto.Sha256d(buf)[:4]
	buf = append(buf, checksum...)
	return base58.Encode(buf)
}

// getWIFInfo parses a WIF string and returns all derived information.
func getWIFInfo(wifString string) (*wifInfoResult, error) {
	privKeyBytes, isTestnet, isCompressed, err := parseWIF(wifString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WIF: %w", err)
	}

	privKey, _ := ec.PrivateKeyFromBytes(privKeyBytes)
	pubKey := privKey.PubKey()

	network := "mainnet"
	if isTestnet {
		network = "testnet"
	}

	// Generate compressed addresses for both networks
	mainnetAddrCompressed, err := script.NewAddressFromPublicKeyWithCompression(pubKey, true, true)
	if err != nil {
		return nil, fmt.Errorf("generating mainnet compressed address: %w", err)
	}
	testnetAddrCompressed, err := script.NewAddressFromPublicKeyWithCompression(pubKey, false, true)
	if err != nil {
		return nil, fmt.Errorf("generating testnet compressed address: %w", err)
	}

	result := &wifInfoResult{
		Input: wifInput{
			WIF:        wifString,
			Network:    network,
			Compressed: isCompressed,
		},
		PublicKey: publicKeyInfo{
			Compressed: hex.EncodeToString(pubKey.Compressed()),
		},
		Mainnet: networkInfo{
			WIF:     keyPair{Compressed: encodeWIF(privKeyBytes, false, true)},
			Address: keyPair{Compressed: mainnetAddrCompressed.AddressString},
		},
		Testnet: networkInfo{
			WIF:     keyPair{Compressed: encodeWIF(privKeyBytes, true, true)},
			Address: keyPair{Compressed: testnetAddrCompressed.AddressString},
		},
	}

	if showUncompr {
		result.PublicKey.Uncompressed = hex.EncodeToString(pubKey.Uncompressed())
		result.Mainnet.WIF.Uncompressed = encodeWIF(privKeyBytes, false, false)
		result.Testnet.WIF.Uncompressed = encodeWIF(privKeyBytes, true, false)

		mainnetAddrUncompressed, err := script.NewAddressFromPublicKeyWithCompression(pubKey, true, false)
		if err != nil {
			return nil, fmt.Errorf("generating mainnet uncompressed address: %w", err)
		}
		testnetAddrUncompressed, err := script.NewAddressFromPublicKeyWithCompression(pubKey, false, false)
		if err != nil {
			return nil, fmt.Errorf("generating testnet uncompressed address: %w", err)
		}
		result.Mainnet.Address.Uncompressed = mainnetAddrUncompressed.AddressString
		result.Testnet.Address.Uncompressed = testnetAddrUncompressed.AddressString
	}

	return result, nil
}

// printJSON outputs the result as formatted JSON.
func printJSON(result *wifInfoResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// c applies ANSI color codes to text if color output is enabled.
func c(color, text string) string {
	if noColor {
		return text
	}
	return color + text + colorReset
}

// printHuman outputs the result in human-readable format.
func printHuman(result *wifInfoResult) {
	line := "────────────────────────────────────────────────────────────────────────"

	fmt.Fprintln(os.Stdout, c(colorWhite, line))
	fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Input WIF:"), c(colorGreen, result.Input.WIF))
	fmt.Fprintf(os.Stdout, "%s  %s\n", c(colorDim, "Network:"), c(colorGreen, result.Input.Network))
	compressed := "yes"
	if !result.Input.Compressed {
		compressed = "no"
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", c(colorDim, "Compressed:"), c(colorGreen, compressed))

	fmt.Fprintf(os.Stdout, "\n%s\n", c(colorDim, "Public Key:"))
	fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "Compressed:"), c(colorGreen, result.PublicKey.Compressed))
	if result.PublicKey.Uncompressed != "" {
		fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "Uncompressed:"), c(colorGreen, result.PublicKey.Uncompressed))
	}

	fmt.Fprintf(os.Stdout, "\n%s\n", c(colorWhite, "MAINNET"))
	fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "WIF:"), c(colorGreen, result.Mainnet.WIF.Compressed))
	fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "Address:"), c(colorGreen, result.Mainnet.Address.Compressed))
	if showUncompr {
		fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "WIF (uncompressed):"), c(colorGreen, result.Mainnet.WIF.Uncompressed))
		fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "Address (uncompressed):"), c(colorGreen, result.Mainnet.Address.Uncompressed))
	}

	fmt.Fprintf(os.Stdout, "\n%s\n", c(colorWhite, "TESTNET"))
	fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "WIF:"), c(colorGreen, result.Testnet.WIF.Compressed))
	fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "Address:"), c(colorGreen, result.Testnet.Address.Compressed))
	if showUncompr {
		fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "WIF (uncompressed):"), c(colorGreen, result.Testnet.WIF.Uncompressed))
		fmt.Fprintf(os.Stdout, "  %s %s\n", c(colorDim, "Address (uncompressed):"), c(colorGreen, result.Testnet.Address.Uncompressed))
	}
	fmt.Fprintln(os.Stdout, c(colorWhite, line))
}

// init initializes the cobra command flags.
func init() {
	rootCmd.Flags().StringVarP(&wif, "wif", "w", "", "WIF private key to analyze")
	rootCmd.Flags().BoolVarP(&jsonFlag, "json", "j", false, "Output in JSON format")
	rootCmd.Flags().BoolVarP(&showUncompr, "uncompressed", "u", false, "Include uncompressed keys, WIFs, and addresses")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
}

// main is the entry point for the wifinfo command.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
