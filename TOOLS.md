# BSV Transaction Tools — User Guide

Fourteen command-line tools for the full Bitcoin SV transaction lifecycle.

## Table of Contents

- [Installation](#installation)
- [Tools Overview](#tools-overview)
  - [keygen — Key Pair Generator](#keygen---key-pair-generator)
  - [wifinfo — WIF Key Inspector](#wifinfo---wif-key-inspector)
  - [addr — Address Validator / Deriver](#addr--address-validator--deriver)
  - [balance — Address Balance Checker](#balance--address-balance-checker)
  - [carve — Transaction Builder](#carve---transaction-builder)
  - [opreturn — OP_RETURN Transaction Builder](#opreturn--op_return-transaction-builder)
  - [broadcast — Transaction Broadcaster](#broadcast---transaction-broadcaster)
  - [txstatus — Status Checker](#txstatus---status-checker)
  - [getraw — Transaction Fetcher](#getraw---transaction-fetcher)
  - [prettytx — Transaction Parser](#prettytx---transaction-parser)
  - [pick — Transaction Field Extractor](#pick---transaction-field-extractor)
  - [decodescript — Script Disassembler](#decodescript--script-disassembler)
  - [signmsg — Message Signer](#signmsg--message-signer)
  - [verifymsg — Message Verifier](#verifymsg--message-verifier)
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
go install ./cmd/keygen
go install ./cmd/wifinfo
go install ./cmd/carve
go install ./cmd/broadcast
go install ./cmd/txstatus
go install ./cmd/getraw
go install ./cmd/prettytx
go install ./cmd/pick
go install ./cmd/addr
go install ./cmd/balance
go install ./cmd/opreturn
go install ./cmd/decodescript
go install ./cmd/signmsg
go install ./cmd/verifymsg
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

### addr — Address Validator / Deriver

Validates a BSV address, or derives mainnet and testnet addresses from a hex-encoded public key.

#### Usage

```bash
addr 1A1zP1eP...                # Validate a mainnet address
addr n2ZNV88u...                # Validate a testnet address
addr 03a34b99f2...              # Derive addresses from a hex pubkey
addr -j 1A1zP1eP...             # JSON output
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--json` | `-j` | Output in JSON format | false |
| `--no-color` | - | Disable colored output | false |

---

### balance — Address Balance Checker

Fetches the confirmed and unconfirmed balance of a BSV address from WhatsOnChain. Accepts an address or a WIF (which is decoded to its address first). Optionally lists individual UTXOs.

#### Usage

```bash
balance 1A1zP1eP...             # Mainnet balance
balance -t mrCDrCyH...          # Testnet balance
balance -u 1A1zP1eP...          # Include the UTXO list
balance -j 1A1zP1eP...          # JSON output
balance <WIF>                   # Balance for the address derived from a WIF
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--testnet` | `-t` | Query the testnet API | false |
| `--utxos` | `-u` | Include individual UTXOs in output | false |
| `--json` | `-j` | Output in JSON format | false |
| `--no-color` | - | Disable colored output | false |

---

### opreturn — OP_RETURN Transaction Builder

Builds and signs a BSV transaction whose sole non-change output is an `OP_RETURN` payload — one or more data pushes.

#### Usage

```bash
# Single push
opreturn -w <WIF> "hello world"

# Multiple pushes (each argument becomes its own push)
opreturn -w <WIF> "prefix" "payload"

# Testnet with custom fee
opreturn -w <WIF> -t -f 50 "payload"
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--wif` | `-w` | WIF private key for funding + signing (required) | — |
| `--testnet` | `-t` | Use testnet | false |
| `--fee-per-kb` | `-f` | Fee per kilobyte in satoshis | 100 |
| `--dust` | `-d` | Dust limit in satoshis | 1 |
| `--debug` | - | Enable debug logging | false |

---

### decodescript — Script Disassembler

Decodes a hex-encoded Bitcoin script (locking or unlocking) to human-readable ASM, with script-type detection (P2PKH, P2PK, multisig, OP_RETURN, etc.). Reads from an argument or stdin.

#### Usage

```bash
decodescript 76a914...88ac      # Decode a hex script
echo 76a914...88ac | decodescript
decodescript -j 76a914...88ac   # JSON output
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--json` | `-j` | Output in JSON format | false |
| `--no-color` | - | Disable colored output | false |

---

### signmsg — Message Signer

Signs a message with a BSV private key using the Bitcoin Signed Message format (compatible with `verifymsg` and standard BSV wallets).

#### Usage

```bash
signmsg -w <WIF> -m "hello"     # Sign a message inline
echo "hello" | signmsg -w <WIF> # Read message from stdin
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--wif` | `-w` | WIF private key for signing (required) | — |
| `--message` | `-m` | Message to sign (falls back to stdin if omitted) | — |

---

### verifymsg — Message Verifier

Verifies a Bitcoin Signed Message signature against an address.

#### Usage

```bash
verifymsg -a <address> -s <base64-sig> -m "hello"
echo "hello" | verifymsg -a <address> -s <base64-sig>
```

#### Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--address` | `-a` | BSV address to verify against (required) | — |
| `--signature` | `-s` | Base64-encoded signature (required) | — |
| `--message` | `-m` | Message to verify (falls back to stdin if omitted) | — |

Exits with a non-zero status if the signature does not verify.

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

### WhatsOnChain (carve, getraw)

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
| `GET /v1/bsv/{net}/address/{addr}/unspent/all` | carve |
| `GET /v1/bsv/{net}/tx/{txid}/hex` | getraw |

### ARC (API key required)

| Endpoint | Used By |
|----------|---------|
| `POST /v1/tx` | broadcast |
| `GET /v1/tx/{txid}` | txstatus, broadcast (monitoring) |

---

## License

See project [LICENSE](LICENSE) file.

---
<!-- docs-sync -->
Documentation up to date as of commit: `5f950f095d10eba5b552100895ba274f23ecb98d`
_This marker is maintained by an automated documentation sync routine. If HEAD has moved past this commit, the routine will re-check for doc drift on its next run._
