package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

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

type decodeResult struct {
	ASM     string `json:"asm"`
	Type    string `json:"type"`
	Size    int    `json:"size"`
	Address string `json:"address,omitempty"`
}

var rootCmd = &cobra.Command{
	Use:   "decodescript [hex]",
	Short: "Decode a Bitcoin script to human-readable ASM",
	Long:  "A command line tool that decodes a hex-encoded Bitcoin script into opcodes, detects script type, and extracts addresses",
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
		cmd.Help() //nolint:errcheck
		return fmt.Errorf("no script hex provided")
	}

	if !cli.IsValidHex(input) {
		return fmt.Errorf("invalid hex string: %s", input)
	}

	scriptBytes, err := hex.DecodeString(input)
	if err != nil {
		return fmt.Errorf("failed to decode hex: %w", err)
	}

	s := script.Script(scriptBytes)

	result := decodeResult{
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

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printHuman(&result)
	return nil
}

func getInput(cmd *cobra.Command, args []string) (string, error) {
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

func c(color, text string) string {
	if noColor {
		return text
	}
	return color + text + colorReset
}

func printHuman(result *decodeResult) {
	fmt.Printf("%s %s\n", c(colorDim, "ASM:"), c(colorGreen, result.ASM))
	fmt.Printf("%s %s\n", c(colorDim, "Type:"), c(colorGreen, result.Type))
	fmt.Printf("%s %s\n", c(colorDim, "Size:"), c(colorGreen, fmt.Sprintf("%d bytes", result.Size)))
	if result.Address != "" {
		fmt.Printf("%s %s\n", c(colorDim, "Address:"), c(colorGreen, result.Address))
	}
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
