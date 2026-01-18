# Privacy-PoS Blockchain - Usage Guide

Complete walkthrough for running your own private blockchain testnet.

## 📋 Prerequisites

### System Requirements
- **OS**: Linux, macOS, or Windows (WSL2)
- **RAM**: 2GB minimum
- **Go**: Version 1.21 or higher
- **Disk**: 1GB free space

### Install Go (if needed)

```bash
# Linux/macOS
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Verify
go version
```

## 🚀 Quick Start (5 Minutes)

### Step 1: Clone & Setup

```bash
# Create project directory
mkdir privacy-blockchain
cd privacy-blockchain

# Create module
go mod init blockchain

# Download dependencies
go get github.com/dgraph-io/badger/v3
go get github.com/libp2p/go-libp2p
go get github.com/libp2p/go-libp2p-pubsub
go get golang.org/x/crypto/ed25519
```

### Step 2: Build Binaries

```bash
# Using Makefile
make build

# Or manually
mkdir -p bin
go build -o bin/node cmd/node/main.go
go build -o bin/wallet cmd/wallet/main.go
```

### Step 3: Generate Validators

```bash
# Generate 3 validator keys
make validators

# Or manually
./bin/wallet generate && mv wallet.json validator1.json
./bin/wallet generate && mv wallet.json validator2.json
./bin/wallet generate && mv wallet.json validator3.json
```

You'll see output like:
```
Wallet generated successfully!
Your stealth address:
  View Key:  a1b2c3d4...
  Spend Key: e5f6g7h8...
```

### Step 4: Update Genesis

Extract public keys from validator files:

```bash
# Linux/macOS with jq
cat validator1.json | jq -r '.SpendKeyPair.PublicKey'
cat validator2.json | jq -r '.SpendKeyPair.PublicKey'
cat validator3.json | jq -r '.SpendKeyPair.PublicKey'
```

Edit `genesis.json` and replace placeholder public keys:

```json
{
  "initial_validators": [
    {
      "public_key": "<VALIDATOR_1_SPEND_KEY>",
      "staked_amount": 100000,
      ...
    },
    ...
  ]
}
```

### Step 5: Launch Testnet

```bash
# Automated launch
make testnet

# Or manually (see Manual Launch section below)
```

Expected output:
```
✅ Testnet is running!

Nodes:
  Node 1 (Bootstrap): PID 12345, Port 9001
  Node 2:             PID 12346, Port 9002
  Node 3:             PID 12347, Port 9003
```

### Step 6: Send Private Transaction

```bash
# Terminal 4 - Generate recipient wallet
./bin/wallet generate

# Show your address
./bin/wallet address

# Send 1000 tokens (example)
./bin/wallet send \
  a1b2c3d4...e5f6g7h8... \
  1000
```

## 📚 Detailed Walkthroughs

### Manual Testnet Launch

**Terminal 1 - Bootstrap Node**
```bash
./bin/node \
  --datadir=./data/node1 \
  --port=9001 \
  --validator=validator1.json \
  --genesis=genesis.json
```

Wait for:
```
Node started successfully
Peer ID: 12D3KooW...
Listening on: [/ip4/127.0.0.1/tcp/9001]
```

Copy the Peer ID and construct bootstrap address:
```
/ip4/127.0.0.1/tcp/9001/p2p/12D3KooW...
```

**Terminal 2 - Node 2**
```bash
./bin/node \
  --datadir=./data/node2 \
  --port=9002 \
  --validator=validator2.json \
  --bootstrap=/ip4/127.0.0.1/tcp/9001/p2p/12D3KooW...
```

**Terminal 3 - Node 3**
```bash
./bin/node \
  --datadir=./data/node3 \
  --port=9003 \
  --validator=validator3.json \
  --bootstrap=/ip4/127.0.0.1/tcp/9001/p2p/12D3KooW...
```

### Transaction Workflow

#### 1. Generate Wallet

```bash
./bin/wallet generate
```

Output:
```
Wallet generated successfully!
Saved to: wallet.json

Your stealth address:
  View Key:  abc123...
  Spend Key: def456...

⚠️  KEEP YOUR WALLET FILE SECURE!
```

**Important**: The stealth address format is:
```
ViewKey:SpendKey
```

#### 2. Check Address

```bash
./bin/wallet address
```

Shows your public stealth address for receiving funds.

#### 3. Send Private Transaction

```bash
# Format: wallet send <recipient_address> <amount>
./bin/wallet send abc123...:def456... 5000
```

Output:
```
Transaction created:
  Amount: 5000
  Fee: 1000
  Hash: 7a8b9c...

Broadcasting to network...
Transaction saved to tx_7a8b9c.json
```

#### 4. Query Balance (Phase 1 Limited)

```bash
./bin/wallet balance
```

Note: Phase 1 requires manual blockchain scanning. Full balance queries coming in Phase 2.

### Validator Operations

#### Stake Tokens

```bash
# Stake 100,000 tokens to become validator
./bin/wallet stake 100000
```

Output:
```
Staking transaction created:
  Validator: abc123...
  Amount: 100000

Staking transaction saved to staking_tx.json
Submit this to the network to become a validator
```

#### Submit Staking Transaction

Phase 1: Manual submission to node
```bash
# Copy staking_tx.json to node and process
# (API endpoint coming in Phase 2)
```

#### Check Validator Status

Monitor node logs:
```bash
tail -f data/node1/logs/node.log
```

Look for:
```
Validator set updated: 4 active validators
Selected as block proposer for height 100
```

## 🔍 Monitoring & Debugging

### Node Logs

```bash
# Real-time logs
tail -f data/node1/logs/node.log

# Search for errors
grep ERROR data/node1/logs/node.log

# Check consensus
grep "Proposing block" data/node1/logs/node.log
```

### Database Inspection

```bash
# Install badger CLI
go install github.com/dgraph-io/badger/v3/badger@latest

# Inspect database
badger info --dir=./data/node1/blockchain.db
```

### Network Diagnostics

Check peer connections:
```bash
# Node should log connected peers
grep "Peer connected" data/node1/logs/node.log
```

### Transaction Pool

```bash
# Check pending transactions in logs
grep "Transaction added to pool" data/node1/logs/node.log
```

## 🧪 Testing Scenarios

### Scenario 1: Basic Transaction Flow

```bash
# 1. Generate sender wallet
./bin/wallet generate
mv wallet.json sender.json

# 2. Generate receiver wallet
./bin/wallet generate
mv wallet.json receiver.json

# 3. Get receiver address
# Extract from receiver.json

# 4. Send transaction
./bin/wallet send <RECEIVER_ADDRESS> 1000

# 5. Monitor blocks
tail -f data/node1/logs/node.log | grep "Block.*finalized"
```

### Scenario 2: Multiple Transactions

```bash
# Send 5 transactions rapidly
for i in {1..5}; do
  ./bin/wallet send <ADDRESS> $((1000 * i))
  sleep 1
done

# Check block production rate
grep "Block.*finalized" data/node1/logs/node.log | tail -20
```

### Scenario 3: Validator Rotation

```bash
# Add 4th validator
./bin/wallet generate
mv wallet.json validator4.json

# Stake tokens
./bin/wallet stake 150000

# Monitor validator set changes
grep "Validator set updated" data/node1/logs/node.log
```

## 🛠️ Troubleshooting

### Problem: Node won't start

**Check**:
```bash
# Port already in use?
lsof -i :9001

# Database corrupted?
rm -rf data/node1
```

### Problem: Peers not connecting

**Solutions**:
1. Check bootstrap address is correct
2. Verify firewall allows connections
3. Use localhost (127.0.0.1) for local testnet
4. Check node logs for connection errors

### Problem: Transactions not confirming

**Check**:
1. Is validator set active?
   ```bash
   grep "active validators" data/node1/logs/node.log
   ```

2. Are blocks being produced?
   ```bash
   grep "Proposing block" data/node1/logs/node.log
   ```

3. Is quorum reached?
   ```bash
   grep "finalized" data/node1/logs/node.log
   ```

### Problem: Balance not updating

**Phase 1 Note**: Balance queries require manual blockchain scanning. This is a known limitation.

**Workaround**:
1. Check transaction confirmed in block
2. Manually scan blockchain database
3. Wait for Phase 2 indexer

## 🧹 Cleanup

### Stop Testnet

```bash
# Find node processes
ps aux | grep "bin/node"

# Kill processes
kill <PID1> <PID2> <PID3>

# Or use Makefile
make clean
```

### Reset Blockchain

```bash
# Remove all data
rm -rf data/

# Keep validator keys, remove blockchain data
rm -rf data/node*/blockchain.db
```

### Full Reset

```bash
# Remove everything
make clean

# Start fresh
make dev
```

## 📊 Performance Tuning

### Increase Block Size

Edit `consensus/engine.go`:
```go
const MaxBlockSize = 1000 // transactions
```

### Adjust Block Time

Edit `consensus/engine.go`:
```go
const BlockTime = 1 * time.Second // faster blocks
```

### Ring Size Configuration

Edit `crypto/ring.go`:
```go
const DefaultRingSize = 10 // more anonymity, slower
```

## 🔐 Security Best Practices

### Testnet Only
- ⚠️ **DO NOT** use for real value
- ⚠️ **DO NOT** expose to public internet
- ⚠️ **DO NOT** reuse testnet keys in production

### Wallet Security
- Store `validator*.json` files securely
- Use strong file permissions: `chmod 600 wallet.json`
- Backup wallet files to encrypted storage
- Never share private keys

### Network Security
- Use VPN for remote nodes
- Configure firewall rules
- Monitor for unusual activity
- Implement rate limiting (Phase 2)

## 📞 Support

### Common Questions

**Q: How do I get testnet tokens?**  
A: Phase 1 uses genesis pre-allocation. Edit `genesis.json` to allocate tokens to your address.

**Q: Can I run more than 3 validators?**  
A: Yes! Generate more validator keys and update genesis. Performance tested up to 100 validators.

**Q: How private are transactions?**  
A: Phase 1: Sender/receiver private, amounts visible. Phase 2 will hide amounts with Bulletproofs.

**Q: Can I use this in production?**  
A: **NO!** Phase 1 uses simplified cryptography for testing only.

### Get Help

1. Check logs: `tail -f data/node1/logs/node.log`
2. Review this guide
3. Open GitHub issue with:
   - Node version
   - Error logs
   - Steps to reproduce

## 🎯 Next Steps

After completing this guide:

1. ✅ **You should have**: 3-node testnet running
2. ✅ **You can**: Send private transactions
3. ✅ **You know**: How to operate validators

**Continue Learning**:
- Read `README.md` for architecture details
- Review source code in `crypto/` for privacy primitives
- Study `consensus/` for PoS implementation
- Check roadmap for Phase 2 features

**Phase 2 Coming Soon**:
- Full cryptographic implementation
- Smart contract support
- Governance features
- Production-ready security

---

**Happy Building! 🚀**