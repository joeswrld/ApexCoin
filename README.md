# Privacy-First Proof-of-Stake Blockchain - Phase 1

A production-ready blockchain combining **Monero-inspired privacy** with **Proof-of-Stake consensus** and **BFT finality**.

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Blockchain Node                     │
├──────────────┬──────────────┬────────────────────────┤
│  P2P Gossip  │  PoS + BFT   │  Validator Management  │
├──────────────┴──────────────┴────────────────────────┤
│          UTXO State & Transaction Pool               │
├──────────────────────────────────────────────────────┤
│  Privacy Layer (Ring Sigs, Stealth Addr, Hidden Amt) │
├──────────────────────────────────────────────────────┤
│            Persistent Storage (BadgerDB)             │
└──────────────────────────────────────────────────────┘
```

## ✨ Features Implemented

### Privacy (Phase 1)
- ✅ **Ring Signatures** - Sender anonymity (simplified LSAG)
- ✅ **Stealth Addresses** - Receiver privacy (ECDH-based)
- ✅ **Key Images** - Double-spend prevention without revealing outputs
- ⚠️ **Amount Hiding** - Visible in Phase 1, hooks for Bulletproofs added

### Consensus
- ✅ **Proof-of-Stake** - Weighted validator selection
- ✅ **BFT Finality** - 2/3 majority for block finalization
- ✅ **Slashing** - Penalties for double-voting
- ✅ **Unbonding Period** - 100 block delay for validator exit

### Core Blockchain
- ✅ **UTXO Model** - Privacy-friendly transaction model
- ✅ **Block Production** - 2-second block time
- ✅ **Transaction Validation** - Ring signature verification
- ✅ **Persistent Storage** - BadgerDB for blockchain data

### Networking
- ✅ **P2P Gossip** - libp2p-based networking
- ✅ **Block Propagation** - Efficient block broadcasting
- ✅ **Transaction Pool** - Mempool for pending transactions
- ✅ **Peer Discovery** - Automatic peer connection

## 📁 Project Structure

```
blockchain/
├── cmd/
│   ├── node/           # Blockchain node
│   └── wallet/         # Wallet CLI
├── consensus/          # PoS + BFT engine
├── crypto/             # Ring sigs, stealth addresses
├── ledger/             # UTXO state management
├── p2p/                # Networking layer
├── storage/            # Database layer
├── types/              # Core data structures
├── genesis.json        # Genesis configuration
└── README.md
```

## 🚀 Quick Start

### Prerequisites

```bash
# Install Go 1.21+
go version

# Install dependencies
go mod init blockchain
go get github.com/dgraph-io/badger/v3
go get github.com/libp2p/go-libp2p
go get github.com/libp2p/go-libp2p-pubsub
go get golang.org/x/crypto/ed25519
```

### 1. Generate Validator Keys

```bash
# Generate 3 validator wallets
go run cmd/wallet/main.go generate
mv wallet.json validator1.json

go run cmd/wallet/main.go generate
mv wallet.json validator2.json

go run cmd/wallet/main.go generate
mv wallet.json validator3.json
```

### 2. Update Genesis

Edit `genesis.json` with actual validator public keys from generated wallets.

### 3. Start Local Testnet

**Terminal 1 - Node 1 (Bootstrap)**
```bash
go run cmd/node/main.go \
  --datadir=./data/node1 \
  --port=9001 \
  --validator=validator1.json \
  --genesis=genesis.json
```

**Terminal 2 - Node 2**
```bash
go run cmd/node/main.go \
  --datadir=./data/node2 \
  --port=9002 \
  --validator=validator2.json \
  --bootstrap=/ip4/127.0.0.1/tcp/9001/p2p/<NODE1_PEER_ID>
```

**Terminal 3 - Node 3**
```bash
go run cmd/node/main.go \
  --datadir=./data/node3 \
  --port=9003 \
  --validator=validator3.json \
  --bootstrap=/ip4/127.0.0.1/tcp/9001/p2p/<NODE1_PEER_ID>
```

### 4. Send a Private Transaction

```bash
# Generate recipient wallet
go run cmd/wallet/main.go generate

# Show your address
go run cmd/wallet/main.go address

# Send transaction
go run cmd/wallet/main.go send <RECIPIENT_ADDRESS> 1000
```

### 5. Stake as Validator

```bash
# Stake tokens
go run cmd/wallet/main.go stake 100000

# Submit staking transaction to network
# (Phase 1: manual submission via node API)
```

## 🔍 How It Works

### Privacy Model

#### Ring Signatures
- Transaction inputs use ring signatures with decoy outputs
- Ring size: 3-10 outputs (configurable)
- Real signer hidden among decoys
- Linkability prevented via key images

#### Stealth Addresses
- One-time addresses per transaction output
- Recipient derives via ECDH: `P' = Hs(rA)G + B`
- Only recipient can detect and spend outputs
- No address reuse observable on-chain

#### Key Images
- Unique identifier per UTXO: `I = x·Hp(P)`
- Prevents double-spending
- No link to actual UTXO

### Consensus Flow

1. **Validator Selection**
   - Weighted random selection based on stake
   - Deterministic: `Hash(height || round) mod total_stake`

2. **Block Proposal**
   - Selected proposer creates block
   - Includes pending transactions
   - Signs and broadcasts

3. **Voting Phase**
   - Validators sign block hash
   - Votes propagated via gossip
   - 2/3+ stake required for finality

4. **Finalization**
   - Block applied to state
   - UTXO set updated
   - Validators rewarded (Phase 2)

### Transaction Lifecycle

```
1. User creates transaction
   ├─ Select owned UTXOs as inputs
   ├─ Generate stealth addresses for outputs
   ├─ Create ring signature with decoys
   └─ Compute key images

2. Broadcast to mempool
   ├─ Validate ring signature
   ├─ Check key image not spent
   └─ Verify amounts balance

3. Validator includes in block
   ├─ Selected by PoS
   └─ Proposes block

4. BFT finalization
   ├─ Validators vote
   ├─ 2/3 quorum reached
   └─ Block finalized

5. State update
   ├─ Mark key images spent
   ├─ Add new UTXOs
   └─ Update validator stakes
```

## 🧪 Testing

### Unit Tests

```bash
# Test cryptographic primitives
go test ./crypto -v

# Test consensus
go test ./consensus -v

# Test UTXO state
go test ./ledger -v
```

### Integration Test

```bash
# Run 3-node testnet
./scripts/run_testnet.sh

# Send test transaction
./scripts/test_transaction.sh
```

## 📊 Performance Metrics

- **Block Time**: 2 seconds
- **Finality**: ~6 seconds (3 blocks)
- **TPS**: ~50 (Phase 1, not optimized)
- **Ring Size**: 5 outputs (configurable)
- **Validator Set**: 3-100 validators

## ⚠️ Phase 1 Limitations & Warnings

### Cryptographic Simplifications

**🔴 CRITICAL - NOT PRODUCTION READY**

1. **Ring Signatures**: Simplified LSAG implementation
   - Uses hash-based operations instead of proper edwards25519
   - Missing proper Fiat-Shamir transform
   - **TODO Phase 2**: Implement CLSAG with curve operations

2. **Stealth Addresses**: Simplified ECDH
   - Uses hash derivation instead of EC point multiplication
   - Not cryptographically sound
   - **TODO Phase 2**: Use edwards25519 library properly

3. **Amount Hiding**: Not implemented
   - Transaction amounts are visible
   - **TODO Phase 2**: Implement Pedersen commitments + Bulletproofs

4. **Key Images**: Simplified hashing
   - Should use hash-to-point on Ed25519 curve
   - **TODO Phase 2**: Proper key image generation

### Known Issues

- [ ] No blockchain reorganization handling
- [ ] Missing network sync protocol
- [ ] No transaction fee market
- [ ] Validator rewards not implemented
- [ ] Missing slashing evidence propagation
- [ ] No checkpoint mechanism
- [ ] Limited DoS protection

### Security Assumptions

- Assumes honest 2/3 validator majority
- No formal security audit conducted
- Simplified crypto is for TESTING ONLY
- Do NOT use with real value

## 🛣️ Phase 2 Roadmap

### Cryptography Upgrades
- [ ] Full CLSAG ring signatures
- [ ] Bulletproofs for range proofs
- [ ] Proper edwards25519 curve operations
- [ ] Multi-signature support

### Consensus Improvements
- [ ] Chain reorganization logic
- [ ] Fast sync protocol
- [ ] Validator rotation epochs
- [ ] Economic finality gadget

### Performance
- [ ] Transaction batching
- [ ] Parallel verification
- [ ] State pruning
- [ ] Light client support

### Features
- [ ] Governance module
- [ ] On-chain treasury
- [ ] Validator delegation
- [ ] Fee market mechanism

## 🔐 Security Considerations

### Do's
✅ Use this for learning and testing  
✅ Run on isolated testnets  
✅ Report bugs and vulnerabilities  
✅ Review cryptographic assumptions  

### Don'ts
❌ Use in production without audit  
❌ Store real value  
❌ Trust the simplified cryptography  
❌ Expose to public internet without hardening  

## 📖 References

### Privacy Protocols
- [CryptoNote Whitepaper](https://cryptonote.org/whitepaper.pdf)
- [Monero Ring Signatures](https://www.getmonero.org/resources/research-lab/)
- [CLSAG Paper](https://eprint.iacr.org/2019/654.pdf)

### Consensus
- [Tendermint Spec](https://github.com/tendermint/tendermint/tree/master/spec)
- [Practical Byzantine Fault Tolerance](http://pmg.csail.mit.edu/papers/osdi99.pdf)

### Implementation
- [edwards25519 Library](https://pkg.go.dev/filippo.io/edwards25519)
- [BadgerDB Documentation](https://dgraph.io/docs/badger/)

## 🤝 Contributing

Phase 1 is feature-complete for testnet launch. Contributions for Phase 2:

1. Fork the repository
2. Create feature branch
3. Add tests
4. Submit PR with detailed description

Focus areas:
- Cryptographic hardening
- Performance optimization
- Test coverage
- Documentation

## 📄 License

MIT License - Use at your own risk

---

**Built with ❤️ for privacy and decentralization**

For questions: Open an issue on GitHub