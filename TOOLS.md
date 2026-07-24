# BSV Transaction Tools — User Guide

> Documentation up to date as of commit: `c302b2f`

Fourteen command-line tools for the full Bitcoin SV transaction lifecycle.

## Table of Contents

- [Installation](#installation)
- [Tools Overview](#tools-overview)
  - [keygen — Key Pair Generator](#keygen---key-pair-generator)
  - [wifinfo — WIF Key Inspector](#wifinfo---wif-key-inspector)
  - [addr — Address Validator & Deriver](#addr---address-validator--deriver)
  - [balance — Balance & UTXO Lister](#balance---balance--utxo-lister)
  - [carve — Transaction Builder](#carve---transaction-builder)
  - [opreturn — OP_RETURN Transaction Builder](#opreturn---op_return-transaction-builder)
  - [broadcast — Transaction Broadcaster](#broadcast---transaction-broadcaster)
  - [txstatus — Status Checker](#txstatus---status-checker)
  - [getraw — Transaction Fetcher](#getraw---transaction-fetcher)
  - [prettytx — Transaction Parser](#prettytx---transaction-parser)
  - [decodescript — Script Disassembler](#decodescript---script-disassembler)
  - [pick — Transaction Field Extractor](#pick---transaction-field-extractor)
  - [signmsg — Bitcoin Signed Message Signer](#signmsg---bitcoin-signed-message-signer)
  - [verifymsg — Bitcoin Signed Message Verifier](#verifymsg---bitcoin-signed-message-verifier)
- [Configuration](#configuration)
- [Examples](#examples)
- [Transaction Size & Fees](#transaction-size--fees)
- [Troubleshooting](#troubleshooting)

---

## Installation

```bash
cd bsv-cmd-line-utils

# Install all tools
go install ./cmd/...

# Or install individually
go install ./cmd/addr
go install ./cmd/balance
go install ./cmd/broadcast
go install ./cmd/carve
go install ./cmd/decodescript
go install ./cmd/getraw
go install ./cmd/keygen
go install ./cmd/opreturn
go install ./cmd/pick
go install ./cmd/prettytx
go install ./cmd/signmsg
go install ./cmd/txstatus
go install ./cmd/verifymsg
go install ./cmd/wifinfo
```

---

## Tools Overview

### keygen — Key Pair Generator

Generates BSV private keys with corresponding public keys and addresses using cryptographically secure randomness.

#### Usage

```bash
keygen                          # Single mainnet key pair
keygen -t                       # Testnet key pair
keygen -c 5                     # Generate 5 key pairs
keygen -j                       # JSON output
keygen -u                       # Uncompressed public key
keygen -t -c 3 -j               # 3 testnet keys in JSON
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--testnet` | `-t` | Generate testnet keys | false |
| `--count` | `-c` | Number of key pairs (1-100) | 1 |
| `--json` | `-j` | Output in JSON format | false |
| `--uncompressed` | `-u` | Use uncompressed public key | false |

#### Output (JSON)

```json
{
  "privateKey": "hex...",
  "publicKey": "hex...",
  "wif": "K...",
  "address": "1...",
  "network": "mainnet",
  "compressed": true
}
```

---

### wifinfo — WIF Key Inspector

Parses a WIF-encoded private key and displays the corresponding public keys, addresses, and WIF representations for both mainnet and testnet. Automatically detects the input network and compression format.

#### Usage

```bash
wifinfo <wif>                   # Parse from argument
wifinfo -w <wif>                # Parse from flag
echo <wif> | wifinfo            # Parse from stdin
wifinfo -j <wif>                # JSON output
wifinfo --no-color <wif>        # Plain output (for scripting)
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--wif` | `-w` | WIF string via flag | - |
| `--json` | `-j` | Output in JSON format | false |
| `--no-color` | - | Disable colored output | false |

#### Output

Shows for both mainnet and testnet:
- Compressed and uncompressed public keys
- Compressed and uncompressed addresses
- Compressed and uncompressed WIF encodings
- Detected input network and compression

---

### addr — Address Validator & Deriver

Validates BSV addresses (showing network and hash160) or derives mainnet/testnet addresses from a public key hex. Auto-detects input type: 66- or 130-hex-char input is treated as a public key, anything else as an address.

#### Usage

```bash
addr <address>                  # Validate address
addr <pubkey_hex>               # Derive addresses from public key
echo <address> | addr           # From stdin
addr -j <address>               # JSON output
addr --no-color <address>       # Plain output (for scripting)
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--json` | `-j` | Output in JSON format | false |
| `--no-color` | - | Disable colored output | false |

#### Output (validation)

Address, `valid` flag, detected network, and hash160.

#### Output (derivation)

Public key plus derived mainnet and testnet addresses.

---

### balance — Balance & UTXO Lister

Checks the balance of a BSV address via WhatsOnChain, and optionally lists its UTXOs. Accepts an address or a WIF; if given a WIF, derives the address automatically.

#### Usage

```bash
balance <address>               # Check balance
balance <wif>                   # Derive address from WIF, then check
echo <address> | balance        # From stdin
balance -t <address>            # Testnet
balance -u <address>            # Also list UTXOs
balance -j <address>            # JSON output
balance --no-color <address>    # Plain output
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--testnet` | `-t` | Use testnet | false |
| `--utxos` | `-u` | Include UTXO list in output | false |
| `--json` | `-j` | Output in JSON format | false |
| `--no-color` | - | Disable colored output | false |

#### Output (JSON)

```json
{
  "address": "1...",
  "confirmed": 12345,
  "unconfirmed": 0,
  "total": 12345,
  "bsv": 0.00012345,
  "utxos": [ { "txHash": "...", "txPos": 0, "value": 12345, "height": 800000 } ]
}
```

Uses WhatsOnChain — no configuration required.

---

### carve — Transaction Builder

Creates and signs BSV transactions with smart UTXO selection and automatic fee estimation.

#### Features
- Largest-first UTXO selection (minimizes inputs)
- Automatic fee estimation with 100 satoshi minimum floor
- Send-all mode (sats=0 sends entire balance minus fees)
- Split payments across multiple equal outputs
- Mainnet/testnet support
- Debug mode for verbose UTXO selection logging
- Dust limit protection

#### Usage

```bash
carve -w <WIF> -a <address> -s 1000              # Send 1000 sats
carve -w <WIF> -a <address>                       # Send all funds
carve -w <WIF> -a <address> -s 1000 -t            # Testnet
carve -w <WIF> -a <address> -s 1000000 -n 10      # Split into 10 equal outputs
carve -w <WIF> -a <address> --debug               # Verbose logging
carve -w <WIF> -a <address> -f 200                # Custom fee rate
```

Outputs raw transaction hex to stdout.

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--wif` | `-w` | Source WIF private key (required) | - |
| `--address` | `-a` | Destination address (required) | - |
| `--sats` | `-s` | Amount in satoshis (0 = send all) | 0 |
| `--testnet` | `-t` | Use testnet | false |
| `--fee-per-kb` | `-f` | Fee per kilobyte in satoshis | 100 |
| `--dust` | `-d` | Dust limit in satoshis | 1 |
| `--num-outputs` | `-n` | Split into N equal outputs | 1 |
| `--debug` | - | Enable debug logging | false |

#### How It Works

1. Derives P2PKH address from WIF
2. Fetches UTXOs from WhatsOnChain API
3. Selects UTXOs using largest-first algorithm
4. Builds transaction (payment + change outputs)
5. Estimates fee based on transaction size
6. Signs all inputs
7. Outputs raw hex to stdout

---

### opreturn — OP_RETURN Transaction Builder

Creates a signed BSV transaction whose sole non-change output is an `OP_RETURN` with the given data pushes. Multiple arguments become multiple pushdata parts. Outputs raw tx hex to stdout, ready to pipe into `broadcast`.

#### Usage

```bash
opreturn -w <WIF> "hello world"                         # Single push
opreturn -w <WIF> "prefix" "payload"                    # Two pushes
opreturn -w <WIF> -t "hello"                            # Testnet
opreturn -w <WIF> "hello" | broadcast                   # Build and broadcast
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--wif` | `-w` | Source WIF private key (required) | - |
| `--testnet` | `-t` | Use testnet | false |
| `--fee-per-kb` | `-f` | Fee per kilobyte in satoshis | 100 |
| `--dust` | `-d` | Dust limit in satoshis | 1 |
| `--debug` | - | Enable debug logging | false |

Uses the same UTXO selection and fee logic as `carve`. All output value comes back as change; the OP_RETURN output itself carries zero satoshis.

---

### broadcast — Transaction Broadcaster

Broadcasts raw transactions to the BSV network using ARC endpoints with optional status monitoring.

#### Usage

```bash
echo <rawtx> | broadcast                # Broadcast to mainnet
broadcast -r <rawtx>                    # From flag
echo <rawtx> | broadcast -t             # Testnet
echo <rawtx> | broadcast -m             # Monitor until final state
echo <rawtx> | broadcast -m -p 10       # Monitor, poll every 10s
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--raw` | `-r` | Raw transaction hex | - |
| `--monitor` | `-m` | Monitor until final state | false |
| `--poll-rate` | `-p` | Polling interval in seconds | 5 |
| `--testnet` | `-t` | Use testnet ARC endpoint | false |

#### Transaction Status Flow

```
RECEIVED → STORED → ANNOUNCED_TO_NETWORK → SEEN_ON_NETWORK → MINED
```

Other states: `REJECTED`, `DOUBLE_SPEND_ATTEMPTED`

Requires `config.yaml` — see [Configuration](#configuration).

---

### txstatus — Status Checker

Checks transaction status on the BSV network via ARC endpoints with optional polling.

#### Usage

```bash
txstatus <txid>                         # Check by argument
txstatus -i <txid>                      # Check by flag
echo <txid> | txstatus                  # From stdin
txstatus <txid> -t                      # Testnet
txstatus <txid> -m                      # Monitor until final
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--txid` | `-i` | Transaction ID | - |
| `--monitor` | `-m` | Monitor until final state | false |
| `--poll-rate` | `-p` | Polling interval in seconds | 5 |
| `--testnet` | `-t` | Use testnet ARC endpoint | false |

Requires `config.yaml` — see [Configuration](#configuration).

---

### getraw — Transaction Fetcher

Fetches raw transaction hex from the WhatsOnChain API.

#### Usage

```bash
getraw <txid>                   # Fetch by argument
getraw -i <txid>                # Fetch by flag
echo <txid> | getraw            # From stdin
getraw <txid> -t                # Testnet
getraw <txid> | prettytx        # Chain with parser
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--txid` | `-i` | Transaction ID | - |
| `--testnet` | `-t` | Use testnet | false |

No configuration required. Uses WhatsOnChain public API (~3 req/sec rate limit).

---

### prettytx — Transaction Parser

Parses raw BSV transactions and displays their components in a human-readable, colorized format.

#### Features
- Colorized terminal output
- Input and output breakdown with script hex
- P2PKH address extraction from scripts
- Satoshi to BSV conversion
- Locktime interpretation (block height vs timestamp)

#### Usage

```bash
echo <rawtx> | prettytx                        # Colorized breakdown
prettytx -r <rawtx>                            # From flag
prettytx --no-color -r <rawtx>                 # Plain (for scripting)
getraw <txid> | prettytx                       # Chain with fetcher
carve -w <WIF> -a <addr> -s 1000 | prettytx   # Preview before broadcast
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--raw` | `-r` | Raw transaction hex | - |
| `--no-color` | - | Disable colored output | false |

#### Output Format

```
================================================================================
TRANSACTION BREAKDOWN
================================================================================

Version: 1 (0x00000001)
In-counter: 1

INPUTS:
--------------------------------------------------------------------------------
Input #0:
  Prev TX ID: abc123...
  Prev Vout: 0
  Script Length: 107 bytes
  Script (hex): 473044022...
  Sequence: 4294967295 (0xffffffff)

Out-counter: 2

OUTPUTS:
--------------------------------------------------------------------------------
Output #0:
  Value: 1000 satoshis (0.00001000 BSV)
  Script Length: 25 bytes
  Script (hex): 76a914...

nLockTime: 0 (0x00000000)
           (Not locked)

================================================================================
Transaction ID: def456...
================================================================================
```

---

### decodescript — Script Disassembler

Decodes a hex-encoded Bitcoin script into opcodes (ASM), detects the script type, and extracts an address when the script is P2PKH.

#### Usage

```bash
decodescript <hex>              # Decode from argument
echo <hex> | decodescript       # From stdin
decodescript -j <hex>           # JSON output
decodescript --no-color <hex>   # Plain output
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--json` | `-j` | Output in JSON format | false |
| `--no-color` | - | Disable colored output | false |

#### Output (JSON)

```json
{
  "asm": "OP_DUP OP_HASH160 ... OP_EQUALVERIFY OP_CHECKSIG",
  "type": "P2PKH",
  "size": 25,
  "address": "1..."
}
```

Pipes cleanly with `pick`:

```bash
getraw <txid> | pick --output-script 0 | decodescript
```

---

### pick — Transaction Field Extractor

Extracts specific parts from raw BSV transactions and outputs them as hex strings, one per line. Designed for pipeline integration.

#### Usage

```bash
# Transaction-level fields
pick <rawtx> --txid                              # Transaction ID
pick <rawtx> --version                           # Version (4-byte LE)
pick <rawtx> --locktime                          # Locktime (4-byte LE)

# Output selectors (repeatable)
pick <rawtx> --output 0                          # First output (serialized)
pick <rawtx> --output-script 0                   # Locking script
pick <rawtx> --output-value 0                    # Value (8-byte LE)

# Input selectors (repeatable)
pick <rawtx> --input 0                           # First input (serialized)
pick <rawtx> --input-script 0                    # Unlocking script
pick <rawtx> --input-prevtxid 0                  # Source txid
pick <rawtx> --input-prevout 0                   # Source output index
pick <rawtx> --input-sequence 0                  # Sequence number

# Multiple selections
pick <rawtx> --txid --output-value 0 --output-value 1

# Pipeline
echo <rawtx> | pick --txid
getraw <txid> | pick --output-script 0
```

Accepts raw hex from argument, `-r` flag, stdin, `file://` path, or HTTP URL.

#### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--raw` | `-r` | Raw transaction hex |
| `--output` | `-o` | Complete serialized output (repeatable) |
| `--output-script` | - | Output locking script (repeatable) |
| `--output-value` | - | Output value in LE hex (repeatable) |
| `--input` | `-i` | Complete serialized input (repeatable) |
| `--input-script` | - | Input unlocking script (repeatable) |
| `--input-prevtxid` | - | Input source txid (repeatable) |
| `--input-prevout` | - | Input source output index (repeatable) |
| `--input-sequence` | - | Input sequence number (repeatable) |
| `--version` | `-v` | Transaction version |
| `--locktime` | `-l` | Transaction locktime |
| `--txid` | - | Transaction ID |

---

### signmsg — Bitcoin Signed Message Signer

Signs a message with a BSV private key using the Bitcoin Signed Message format. Prints the base64-encoded signature to stdout.

#### Usage

```bash
signmsg -w <WIF> -m "hello"                     # Sign from flag
echo "hello" | signmsg -w <WIF>                 # Sign from stdin
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--wif` | `-w` | WIF private key for signing (required) | - |
| `--message` | `-m` | Message to sign | - |

Pairs with `verifymsg` for message authentication:

```bash
SIG=$(echo "hello" | signmsg -w <WIF>)
echo "hello" | verifymsg -a <address> -s "$SIG"
```

---

### verifymsg — Bitcoin Signed Message Verifier

Verifies a Bitcoin Signed Message signature against a BSV address. Exits 0 on valid signature, 1 on invalid.

#### Usage

```bash
verifymsg -a <address> -s <base64_sig> -m "hello"
echo "hello" | verifymsg -a <address> -s <base64_sig>
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--address` | `-a` | BSV address to verify against (required) | - |
| `--signature` | `-s` | Base64-encoded signature (required) | - |
| `--message` | `-m` | Message to verify | - |

Prints `Valid` on success or `Invalid: <reason>` on failure.

---

## Configuration

### ARC Configuration (broadcast, txstatus)

Create `config.yaml` in the executable directory or current working directory:

```yaml
arc-mainnet:
  url: "https://api.taal.com"
  api_key: "your_mainnet_key"
  timeout: "30s"

arc-testnet:
  url: "https://arc-test.taal.com"
  api_key: "your_testnet_key"
  timeout: "30s"

polling:
  interval: "3s"
  max_retries: 10
  backoff_factor: 1.5

targets:
  default: "SEEN_BY_NETWORK"
  wait_for_mining: false
```

### WhatsOnChain (addr, balance, carve, getraw, opreturn)

No configuration needed. Uses public API endpoints:
- Mainnet: `https://api.whatsonchain.com/v1/bsv/main/`
- Testnet: `https://api.whatsonchain.com/v1/bsv/test/`

Rate limit: ~3 requests/second.

---

## Examples

### Complete Transaction Workflow

```bash
# 1. Generate a key
keygen -t -j > key.json

# 2. Fund the address (faucet or transfer)

# 3. Create a transaction
carve -w $(jq -r .wif key.json) -a <dest> -s 1000 -t > tx.hex

# 4. Preview
cat tx.hex | prettytx

# 5. Broadcast with monitoring
cat tx.hex | broadcast -t -m

# 6. Check later
txstatus <txid> -t
```

### Pipeline Composition

```bash
# Create, view, and broadcast in one command
carve -w <WIF> -a <addr> -s 5000 | tee >(prettytx) | broadcast -m

# Extract locking script from first output of a known tx
getraw <txid> | pick --output-script 0

# Get txid and all output values
getraw <txid> | pick --txid --output-value 0 --output-value 1
```

### Key Inspection

```bash
# Get mainnet address from a testnet WIF
wifinfo -j <testnet_wif> | jq '.mainnet.address.compressed'

# Verify a WIF matches an expected address
wifinfo <wif> | grep <expected_address>
```

### Batch Processing

```bash
# Inspect multiple transactions
cat txids.txt | while read txid; do
  echo "=== $txid ==="
  getraw "$txid" | prettytx
done

# Extract all output scripts from a list of txids
cat txids.txt | while read txid; do
  getraw "$txid" | pick --output-script 0
done
```

### Send All Funds

```bash
# Sweep entire balance minus fees
carve -w <WIF> -a <dest> | broadcast -m
```

### Split Outputs

```bash
# Split 1 BSV into 10 equal outputs
carve -w <WIF> -a <addr> -s 100000000 -n 10 | broadcast -m
```

---

## Transaction Size & Fees

| Component | Size (bytes) |
|-----------|-------------|
| Base overhead | ~10 |
| Per input (P2PKH) | ~148 |
| Per output (P2PKH) | ~34 |

**Example**: 2 inputs, 2 outputs = 10 + (2 × 148) + (2 × 34) = **374 bytes**

Fee formula: `max(100, (size × feePerKB) / 1000)`

Default fee rate: 100 sat/KB. Minimum floor: 100 sats.

BSV fees are very low (~0.05 sat/byte). A typical 1-in-2-out transaction costs ~100 sats.

---

## Troubleshooting

### "No UTXOs found for address"

Address has no unspent outputs. Verify funds on [WhatsOnChain](https://whatsonchain.com) and check you're on the correct network (mainnet vs testnet).

### "Insufficient funds"

Not enough satoshis to cover amount + fees. Use `--debug` with carve to see available UTXOs and fee calculation.

### "Transaction rejected by network"

Check the transaction with `prettytx` for issues. Common causes: double spend, invalid script, insufficient fee. Verify UTXOs haven't been spent elsewhere.

### "ARC error: unauthorized"

Invalid or missing API key in `config.yaml`. Check the key and ensure the config file is in the correct location.

### General Tips

- Use `--debug` with carve for verbose UTXO selection and fee logging
- Always preview with `prettytx` before broadcasting
- Use `-m` (monitor) to watch transaction progression
- Use `-t` (testnet) for experimentation
- Pipe through `--no-color` when capturing output in scripts

---

## Security

- **Never commit WIF keys** to version control
- **Never share WIF keys** — they control funds directly
- Store keys in environment variables or secure vaults:
  ```bash
  export WIF=$(cat ~/.secure/wallet.wif)
  carve -w "$WIF" -a <address> -s 1000
  ```
- Protect `config.yaml` with ARC API keys: `chmod 600 config.yaml`
- **Use testnet** (`-t`) for experimentation

---

## API Endpoints Used

### WhatsOnChain (no auth required)

| Endpoint | Used By |
|----------|---------|
| `GET /v1/bsv/{net}/address/{addr}/unspent/all` | carve, opreturn, balance |
| `GET /v1/bsv/{net}/address/{addr}/balance` | balance |
| `GET /v1/bsv/{net}/tx/{txid}/hex` | getraw |

### ARC (API key required)

| Endpoint | Used By |
|----------|---------|
| `POST /v1/tx` | broadcast |
| `GET /v1/tx/{txid}` | txstatus, broadcast (monitoring) |

---

## License

See project [LICENSE](LICENSE) file.
