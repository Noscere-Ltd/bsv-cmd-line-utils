# BSV Command Line Tools

A suite of command-line utilities for the full Bitcoin SV transaction lifecycle: key generation, transaction building, broadcasting, inspection, field extraction, and status tracking.

All tools are single Go binaries with no runtime dependencies. Designed for Unix-style pipeline composition.

## Tools

| Tool | Description |
|------|-------------|
| **addr** | Validates BSV addresses or derives addresses from a public key |
| **balance** | Checks address balance and lists UTXOs via WhatsOnChain |
| **broadcast** | Broadcasts raw transactions to the BSV network via ARC with optional monitoring |
| **carve** | Creates and signs BSV transactions with smart UTXO selection and fee estimation |
| **decodescript** | Decodes hex-encoded scripts to human-readable ASM with type detection |
| **getraw** | Fetches raw transaction hex from WhatsOnChain |
| **keygen** | Generates BSV key pairs (mainnet/testnet, compressed/uncompressed, JSON output) |
| **opreturn** | Creates signed transactions with OP_RETURN data outputs |
| **pick** | Extracts specific fields from raw transactions for pipeline processing |
| **prettytx** | Parses and displays raw transactions in human-readable colorized format |
| **signmsg** | Signs messages using Bitcoin Signed Message format |
| **txstatus** | Checks transaction status via ARC with optional polling until final state |
| **verifymsg** | Verifies Bitcoin Signed Message signatures |
| **wifinfo** | Inspects a WIF private key — shows pubkeys, addresses, and WIFs for both networks |

## Installation

```bash
git clone https://github.com/Noscere-Ltd/bsv-cmd-line-utils.git
cd bsv-cmd-line-utils

# Install all tools
go install ./cmd/...
```

## Quick Start

### Generate a key pair

```bash
keygen                          # Mainnet
keygen -t -j                    # Testnet, JSON output
```

### Inspect an existing key

```bash
wifinfo <WIF>                   # Colorized output
wifinfo -j <WIF>                # JSON output
```

### Create, preview, and broadcast a transaction

```bash
# Build a transaction
carve -w <WIF> -a <address> -s 1000 > tx.hex

# Preview it
cat tx.hex | prettytx

# Broadcast with monitoring
cat tx.hex | broadcast -m
```

### Fetch and inspect an existing transaction

```bash
getraw <txid> | prettytx
```

### Extract specific fields

```bash
getraw <txid> | pick --txid --output-value 0
getraw <txid> | pick --output-script 0
```

### Testnet workflow

```bash
keygen -t -j                                              # Generate testnet key
carve -w <WIF> -a <address> -s 1000 -t | broadcast -t -m  # Build + broadcast on testnet
```

### Pipeline composition

```bash
# Create, view, and broadcast in one command
carve -w <WIF> -a <address> -s 5000 | tee >(prettytx) | broadcast -m

# Batch inspect transactions
cat txids.txt | while read txid; do getraw "$txid" | prettytx; done
```

## Documentation

See [TOOLS.md](TOOLS.md) for comprehensive documentation including:
- Detailed usage and flags for each tool
- Configuration options
- Advanced examples and workflows
- Fee estimation
- Troubleshooting guide

## Configuration

`broadcast` and `txstatus` require a `config.yaml` file (in executable dir or cwd):

```yaml
arc-mainnet:
  url: "https://api.taal.com"
  api_key: "your_api_key_here"
  timeout: "30s"

arc-testnet:
  url: "https://arc-test.taal.com"
  api_key: "your_testnet_api_key_here"
  timeout: "30s"

polling:
  interval: "3s"
  max_retries: 10
  backoff_factor: 1.5
```

Other tools (`carve`, `getraw`, `balance`) query WhatsOnChain directly — no API key required.

## Project Structure

```
bsv-cmd-line-utils/
├── cmd/
│   ├── addr/          # Address validator & deriver
│   ├── balance/       # Balance checker & UTXO lister
│   ├── broadcast/     # Transaction broadcaster (ARC)
│   ├── carve/         # Transaction builder (UTXO selection + signing)
│   ├── decodescript/  # Script disassembler
│   ├── getraw/        # Transaction fetcher (WhatsOnChain)
│   ├── keygen/        # Key pair generator
│   ├── opreturn/      # OP_RETURN transaction builder
│   ├── pick/          # Transaction field extractor
│   ├── prettytx/      # Transaction parser/visualizer
│   ├── signmsg/       # Message signer (BSM)
│   ├── txstatus/      # Status checker (ARC)
│   ├── verifymsg/     # Message verifier (BSM)
│   └── wifinfo/       # WIF key inspector
├── internal/
│   ├── arc/          # ARC client
│   ├── cli/          # Shared CLI utilities
│   └── config/       # Configuration loading
├── skill/            # OpenClaw agent skill
├── TOOLS.md          # Detailed documentation
└── README.md
```

## Dependencies

- [go-sdk](https://github.com/bsv-blockchain/go-sdk) — BSV SDK for Go
- [go-whatsonchain](https://github.com/mrz1836/go-whatsonchain) — WhatsOnChain API client
- [cobra](https://github.com/spf13/cobra) — CLI framework

## Security

- **Never commit WIF private keys** to version control
- Protect ARC API keys in `config.yaml` (`chmod 600 config.yaml`)
- Use testnet (`-t` flag) for experimentation
- Store WIF keys in environment variables or secure vaults

For security issues, see [SECURITY.md](.github/SECURITY.md).

## License

[MIT](LICENSE)
