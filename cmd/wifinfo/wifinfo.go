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
	"errors"
	"fmt"
	"os"

	base58 "github.com/bsv-blockchain/go-sdk/compat/base58"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	crypto "github.com/bsv-blockchain/go-sdk/primitives/hash"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/spf13/cobra"

	"github.com/Noscere-Ltd/bsv-cmd-line-utils/internal/cli"
)

var (
	errNoWIFProvided        = errors.New("no WIF provided")
	errInvalidCompressFlag  = errors.New("invalid compression flag")
	errInvalidWIFLength     = errors.New("invalid WIF length")
	errUnknownNetworkPrefix = errors.New("unknown network prefix")
	errInvalidWIFChecksum   = errors.New("invalid WIF checksum")
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

// flags holds the command-line flags for the wifinfo tool.
type flags struct {
	wif         string // WIF string provided via flag
	jsonFlag    bool   // Output in JSON format
	showUncompr bool   // Include uncompressed keys, WIFs, and addresses
	noColor     bool   // Disable colored output
}

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

// newRootCmd builds the cobra command for the wifinfo tool.
func newRootCmd() *cobra.Command {
	f := &flags{}

	cmd := &cobra.Command{
		Use:   "wifinfo [wif]",
		Short: "Display mainnet and testnet details for a BSV private key in WIF format",
		Long:  "A command line tool that parses a WIF-encoded BSV private key and displays public keys, addresses, and WIF representations for both mainnet and testnet",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args, f)
		},
	}

	cmd.Flags().StringVarP(&f.wif, "wif", "w", "", "WIF private key to analyze")
	cmd.Flags().BoolVarP(&f.jsonFlag, "json", "j", false, "Output in JSON format")
	cmd.Flags().BoolVarP(&f.showUncompr, "uncompressed", "u", false, "Include uncompressed keys, WIFs, and addresses")
	cmd.Flags().BoolVar(&f.noColor, "no-color", false, "Disable colored output")

	return cmd
}

// run handles the main execution flow.
func run(cmd *cobra.Command, args []string, f *flags) error {
	wifString, err := getWIF(cmd, args, f)
	if err != nil {
		return err
	}

	if wifString == "" {
		_ = cmd.Help()
		return errNoWIFProvided
	}

	result, err := getWIFInfo(wifString, f.showUncompr)
	if err != nil {
		return err
	}

	if f.jsonFlag {
		return printJSON(result)
	}

	printHuman(result, f.showUncompr, f.noColor)

	return nil
}

// getWIF retrieves the WIF string from argument, flag, or stdin.
func getWIF(_ *cobra.Command, args []string, f *flags) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	if f.wif != "" {
		return f.wif, nil
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return cli.ReadHexFromReader(os.Stdin)
	}

	return "", nil
}

// parseWIF decodes and validates a WIF string, returning the private key bytes,
// network, and compression flag.
func parseWIF(wifString string) (privKeyBytes []byte, isTestnet bool, isCompressed bool, err error) {
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
			return nil, false, false, fmt.Errorf("%w: 0x%02x", errInvalidCompressFlag, decoded[33])
		}

		isCompressed = true
	case 1 + privateKeyLen + 4:
		isCompressed = false
	default:
		return nil, false, false, fmt.Errorf("%w: %d bytes", errInvalidWIFLength, decodedLen)
	}

	// Detect network
	switch decoded[0] {
	case mainnetWIFPrefix:
		isTestnet = false
	case testnetWIFPrefix:
		isTestnet = true
	default:
		return nil, false, false, fmt.Errorf("%w: 0x%02x", errUnknownNetworkPrefix, decoded[0])
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
		return nil, false, false, errInvalidWIFChecksum
	}

	privKeyBytes = decoded[1 : 1+privateKeyLen]

	return privKeyBytes, isTestnet, isCompressed, nil
}

// encodeWIF generates a WIF string from a private key with the given network and compression.
func encodeWIF(privKeyBytes []byte, isTestnet bool, isCompressed bool) string {
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
func getWIFInfo(wifString string, showUncompr bool) (*wifInfoResult, error) {
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
func c(noColor bool, color, text string) string {
	if noColor {
		return text
	}

	return color + text + colorReset
}

// printHuman outputs the result in human-readable format.
func printHuman(result *wifInfoResult, showUncompr, noColor bool) {
	line := "────────────────────────────────────────────────────────────────────────"

	w := os.Stdout
	fmt.Fprintln(w, c(noColor, colorWhite, line))                                                              //nolint:errcheck // terminal output, not worth failing on
	fmt.Fprintf(w, "%s %s\n", c(noColor, colorDim, "Input WIF:"), c(noColor, colorGreen, result.Input.WIF))    //nolint:errcheck // terminal output, not worth failing on
	fmt.Fprintf(w, "%s  %s\n", c(noColor, colorDim, "Network:"), c(noColor, colorGreen, result.Input.Network)) //nolint:errcheck // terminal output, not worth failing on

	compressed := "yes"
	if !result.Input.Compressed {
		compressed = "no"
	}

	fmt.Fprintf(w, "%s %s\n", c(noColor, colorDim, "Compressed:"), c(noColor, colorGreen, compressed)) //nolint:errcheck // terminal output, not worth failing on

	fmt.Fprintf(w, "\n%s\n", c(noColor, colorDim, "Public Key:"))                                                         //nolint:errcheck // terminal output, not worth failing on
	fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "Compressed:"), c(noColor, colorGreen, result.PublicKey.Compressed)) //nolint:errcheck // terminal output, not worth failing on

	if result.PublicKey.Uncompressed != "" {
		fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "Uncompressed:"), c(noColor, colorGreen, result.PublicKey.Uncompressed)) //nolint:errcheck // terminal output, not worth failing on
	}

	fmt.Fprintf(w, "\n%s\n", c(noColor, colorWhite, "MAINNET"))                                                              //nolint:errcheck // terminal output, not worth failing on
	fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "WIF:"), c(noColor, colorGreen, result.Mainnet.WIF.Compressed))         //nolint:errcheck // terminal output, not worth failing on
	fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "Address:"), c(noColor, colorGreen, result.Mainnet.Address.Compressed)) //nolint:errcheck // terminal output, not worth failing on

	if showUncompr {
		fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "WIF (uncompressed):"), c(noColor, colorGreen, result.Mainnet.WIF.Uncompressed))         //nolint:errcheck // terminal output, not worth failing on
		fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "Address (uncompressed):"), c(noColor, colorGreen, result.Mainnet.Address.Uncompressed)) //nolint:errcheck // terminal output, not worth failing on
	}

	fmt.Fprintf(w, "\n%s\n", c(noColor, colorWhite, "TESTNET"))                                                              //nolint:errcheck // terminal output, not worth failing on
	fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "WIF:"), c(noColor, colorGreen, result.Testnet.WIF.Compressed))         //nolint:errcheck // terminal output, not worth failing on
	fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "Address:"), c(noColor, colorGreen, result.Testnet.Address.Compressed)) //nolint:errcheck // terminal output, not worth failing on

	if showUncompr {
		fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "WIF (uncompressed):"), c(noColor, colorGreen, result.Testnet.WIF.Uncompressed))         //nolint:errcheck // terminal output, not worth failing on
		fmt.Fprintf(w, "  %s %s\n", c(noColor, colorDim, "Address (uncompressed):"), c(noColor, colorGreen, result.Testnet.Address.Uncompressed)) //nolint:errcheck // terminal output, not worth failing on
	}

	fmt.Fprintln(w, c(noColor, colorWhite, line)) //nolint:errcheck // terminal output, not worth failing on
}

// main is the entry point for the wifinfo command.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
