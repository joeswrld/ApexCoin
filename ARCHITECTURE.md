# Technical Architecture

Deep dive into the Privacy-PoS blockchain architecture.

## 🏗️ System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Layer                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  Node CLI    │  │ Wallet CLI   │  │  RPC API     │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                      Node Core                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │           Transaction Pool (Mempool)                 │   │
│  └─────────────────────────────────────────────────────┘   │
│                              │                               │
│  ┌──────────────┬────────────────────┬──────────────────┐  │
│  │  Consensus   │   State Manager    │   Validator Mgr  │  │
│  │  PoS + BFT   │   UTXO Set         │   Stake/Slash    │  │
│  └──────────────┴────────────────────┴──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Privacy Layer                              │
│  ┌────────────────┬─────────────────┬──────────────────┐   │
│  │ Ring Sigs      │ Stealth Addr    │  Amount Hiding   │   │
│  │ (LSAG)         │ (ECDH)          │  (Pedersen)      │   │
│  └────────────────┴─────────────────┴──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Network Layer                              │
│  ┌────────────────┬─────────────────┬──────────────────┐   │
│  │ P2P Gossip     │ Peer Discovery  │  Message Router  │   │
│  │ (libp2p)       │ (DHT)           │  (PubSub)        │   │
│  └────────────────┴─────────────────┴──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Storage Layer                              │
│  ┌────────────────┬─────────────────┬──────────────────┐   │
│  │ Blockchain DB  │  State DB       │   Index DB       │   │
│  │ (BadgerDB)     │  (BadgerDB)     │   (BadgerDB)     │   │
│  └────────────────┴─────────────────┴──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## 📦 Module Breakdown

### 1. Types (`types/types.go`)

**Core Data Structures**

```go
// Block structure
Block {
    Header      BlockHeader
    Transactions []*Transaction
    Validators  []ValidatorSignature
}

// Privacy-focused transaction
Transaction {
    Inputs      []*TxInput
    Outputs     []*TxOutput
    RingSignature *RingSignature
    RangeProofs [][]byte
}

// UTXO with stealth addressing
TxOutput {
    Amount      uint64
    StealthAddr Address
    TxPublicKey PublicKey
}
```

**Design Decisions**:
- UTXO model for privacy (vs account model)
- Key images instead of UTXO references
- Stealth addresses for receiver anonymity
- BFT validator signatures for finality

### 2. Cryptography (`crypto/`)

#### Keys (`crypto/keys.go`)

**Stealth Address Scheme**:
```
Sender generates:
  r = random scalar (ephemeral key)
  R = r·G (ephemeral public key)
  
Shared secret:
  S = r·A (sender uses recipient's view key A)
  
One-time address:
  P' = Hs(S)·G + B
  where B is recipient's spend key
  
Recipient scans:
  S = a·R (recipient uses their view private key a)
  Checks if P' matches any output
```

**Phase 1 Simplification**:
```go
// Uses hash-based derivation instead of EC ops
sharedSecret := Hash(ephemeral_priv || recipient_view_pub)
oneTimeKey := Hash(sharedSecret || recipient_spend_pub)
```

**Phase 2 Upgrade**:
```go
// Proper edwards25519 scalar multiplication
import "filippo.io/edwards25519"

sharedSecret := edwards25519.ScalarMult(ephemeral_priv, recipient_view_pub)
oneTimeKey := edwards25519.PointAdd(
    edwards25519.ScalarBaseMult(Hs(sharedSecret)),
    recipient_spend_pub
)
```

#### Ring Signatures (`crypto/ring.go`)

**LSAG (Linkable Spontaneous Anonymous Group)**:

```
Signer has:
  - Private key: x
  - Public key: P
  - Ring: [P1, P2, ..., Pn] (including P)
  - Key image: I = x·Hp(P)

Signature (c, r1, ..., rn, I):
  1. Choose random α
  2. For each ring member i:
     - If real signer: Li = α·G, Ri = α·Hp(Pi)
     - Else: Li = ri·G + ci·Pi, Ri = ri·Hp(Pi) + ci·I
  3. Challenge: c = H(m, L1, R1, ..., Ln, Rn)
  4. Response for real signer: ri = α - ci·x
  
Verification:
  Recompute challenge, check all equations hold
```

**Phase 1 Implementation**:
- Simplified challenge computation
- Hash-based responses (not proper scalar arithmetic)
- Functional but not cryptographically sound

**Phase 2 TODO**:
- Implement CLSAG (more efficient)
- Use edwards25519 for scalar operations
- Proper Fiat-Shamir transform

### 3. Ledger (`ledger/state.go`)

**UTXO Set Management**:

```go
State {
    utxos          map[string]*UTXO
    spentKeyImages map[PublicKey]bool
    validators     map[PublicKey]*ValidatorState
}
```

**Key Operations**:

1. **Add Transaction**:
   ```
   - Verify key images not spent
   - Verify ring signatures
   - Mark key images as spent
   - Add new outputs to UTXO set
   ```

2. **Query Balance**:
   ```
   - Scan all UTXOs
   - For each, attempt to derive spend key
   - If successful, output belongs to wallet
   - Sum amounts
   ```

3. **Select Inputs**:
   ```
   - Find owned UTXOs
   - Select enough to cover amount + fee
   - Generate key images
   - Create change output if needed
   ```

**State Commitment**:
```go
// Merkle root of UTXO set
StateRoot = Hash(UTXO_1 || UTXO_2 || ... || UTXO_n)
```

### 4. Consensus (`consensus/engine.go`)

**Proof-of-Stake with BFT Finality**

#### Validator Selection

```go
// Weighted random based on stake
SelectProposer(height, round) {
    seed = Hash(height || round)
    selection = seed % total_stake
    
    cumulative = 0
    for each validator {
        cumulative += validator.stake
        if selection < cumulative {
            return validator
        }
    }
}
```

**Properties**:
- Deterministic (all nodes agree)
- Proportional to stake
- Unpredictable before height known

#### Block Proposal

```
1. Proposer selected for (height, round)
2. Proposer creates block:
   - Include pending transactions
   - Compute Merkle roots
   - Sign block header
3. Broadcast to network
```

#### Voting Phase

```
1. Validators receive block
2. Each validator:
   - Validates block
   - Signs block hash
   - Broadcasts vote
3. Collect votes until 2/3 quorum
```

#### Finalization

```
IF votes >= 2/3 total stake:
    - Block is final
    - Cannot be reverted
    - Apply to state
    - Advance to next height
ELSE:
    - Increment round
    - New proposer selected
    - Repeat
```

**Slashing Conditions**:

1. **Double-Voting**:
   ```
   IF validator signs two blocks at same height:
       Slash 10% of stake
       Increment slash counter
   ```

2. **Downtime**:
   ```
   IF validator misses N consecutive proposals:
       Mark inactive
       Start unbonding period
   ```

3. **Invalid Block**:
   ```
   IF validator proposes invalid block:
       Slash 5% of stake
       Temporary suspension
   ```

### 5. Networking (`p2p/network.go`)

**libp2p-based P2P Network**

#### Message Types

```go
BlockTopic:
  - New block proposals
  - Finalized blocks

TxTopic:
  - Pending transactions
  - Broadcast to mempool

VoteTopic:
  - Validator votes
  - BFT consensus messages
```

#### Gossip Protocol

```
1. Node receives message
2. Validate message
3. Process locally
4. Forward to random subset of peers
5. Track seen messages (prevent loops)
```

**Peer Management**:
```go
- Bootstrap from seed nodes
- DHT for peer discovery
- Maintain 10-50 peer connections
- Heartbeat every 30 seconds
- Disconnect inactive peers
```

### 6. Storage (`storage/db.go`)

**BadgerDB Key-Value Store**

**Schema**:

```
# Blocks
b:<height>          -> Block (by height)
h:<block_hash>      -> Block (by hash)

# Transactions
t:<tx_hash>         -> Transaction

# State
utxo:<key>          -> UTXO
key_image:<image>   -> bool (spent)
validator:<pubkey>  -> ValidatorState

# Metadata
latest_height       -> uint64
genesis             -> GenesisConfig
```

**Indexing** (Phase 2):
```
# Transaction index
tx_by_addr:<addr>   -> []TxHash

# Balance cache
balance:<addr>      -> uint64

# UTXO index by amount (for decoy selection)
utxo_by_amount:<amt> -> []UTXO
```

## 🔄 Transaction Lifecycle

### 1. Creation (Wallet)

```
User initiates send:
├─ Wallet scans UTXO set for owned outputs
├─ Selects inputs to cover amount + fee
├─ Generates stealth address for recipient
├─ Creates ring signature with decoys
├─ Builds transaction structure
└─ Broadcasts to network
```

### 2. Propagation (P2P)

```
Transaction enters network:
├─ Node receives via TxTopic
├─ Validates ring signature
├─ Checks key images not spent
├─ Verifies amounts balance
├─ Adds to mempool
└─ Gossips to peers
```

### 3. Inclusion (Consensus)

```
Validator proposes block:
├─ Selected by PoS algorithm
├─ Picks transactions from mempool
├─ Orders by fee (high to low)
├─ Constructs block
└─ Broadcasts proposal
```

### 4. Finalization (BFT)

```
Consensus process:
├─ Validators vote on block
├─ Collect signatures
├─ Verify 2/3 quorum reached
├─ Apply block to state
└─ Mark as finalized
```

### 5. Confirmation (State)

```
State update:
├─ Mark key images as spent
├─ Add new UTXOs to set
├─ Update state root
├─ Persist to database
└─ Emit confirmation event
```

## 🔐 Security Model

### Assumptions

1. **Honest Majority**: >2/3 validators are honest
2. **Network Synchrony**: Messages delivered within bounded time
3. **Cryptographic Security**: Ed25519, SHA-256 are secure

### Threat Model

**What we protect against**:
- ✅ Transaction graph analysis
- ✅ Address clustering
- ✅ Amount tracing
- ✅ Double-spending
- ✅ Block withholding
- ✅ Nothing-at-stake attacks

**What we DON'T protect against** (Phase 1):
- ❌ Timing analysis
- ❌ Network-level correlation
- ❌ Quantum attacks
- ❌ Advanced statistical analysis

### Privacy Guarantees

**Sender Privacy**:
- Ring signatures hide sender among N decoys
- Anonymity set size = ring size
- Unlinkable unless <50% of ring is known

**Receiver Privacy**:
- Stealth addresses prevent address reuse
- Cannot link multiple payments to same recipient
- Only recipient can detect ownership

**Amount Privacy** (Phase 2):
- Pedersen commitments hide amounts
- Bulletproofs prove amounts in valid range
- Homomorphic properties enable validation

## 🎯 Performance Characteristics

### Throughput

```
Theoretical Maximum:
- Block time: 2 seconds
- Max transactions/block: 1000
- TPS: 500

Realistic (Phase 1):
- Ring signature verification: ~50 tx/sec
- Network latency: 2x overhead
- Practical TPS: ~25
```

### Latency

```
Transaction Confirmation:
- Mempool acceptance: <1 second
- Block inclusion: ~2 seconds (next block)
- BFT finalization: ~4 seconds (2 block confirmations)
- Total: ~6 seconds for finality
```

### Storage

```
Per Transaction:
- Ring signature: ~5 KB
- Stealth outputs: ~128 bytes each
- Total: ~6 KB average

Per Block:
- Header: 256 bytes
- 100 transactions: ~600 KB
- Validator signatures: ~2 KB
- Total: ~602 KB

Annual Growth (100 tx/block, 2s blocks):
- Blocks/year: 15,768,000
- Data: ~9.5 TB/year
```

## 🔬 Future Optimizations

### Phase 2 Improvements

1. **Cryptography**:
   - CLSAG instead of LSAG (smaller signatures)
   - Bulletproofs for range proofs
   - Signature aggregation

2. **Consensus**:
   - Fast sync protocol
   - State pruning
   - Checkpoint finality

3. **Storage**:
   - UTXO set compression
   - Historical data pruning
   - Light client support

4. **Networking**:
   - Compact block relay
   - Transaction relay optimization
   - Dandelion++ for transaction privacy

### Scaling Roadmap

**Layer 1**:
- Increase block size → 2x TPS
- Parallel validation → 3x TPS
- Target: 150 TPS

**Layer 2**:
- Payment channels
- Rollups for smart contracts
- Sidechains for specialized apps

## 📚 References

### Academic Papers

1. **Ring Signatures**:
   - [LSAG Paper](https://eprint.iacr.org/2004/027.pdf)
   - [CLSAG Paper](https://eprint.iacr.org/2019/654.pdf)

2. **Stealth Addresses**:
   - [CryptoNote Whitepaper](https://cryptonote.org/whitepaper.pdf)

3. **Consensus**:
   - [Tendermint](https://tendermint.com/static/docs/tendermint.pdf)
   - [PBFT](http://pmg.csail.mit.edu/papers/osdi99.pdf)

### Implementation References

- [Monero Source](https://github.com/monero-project/monero)
- [Cosmos SDK](https://github.com/cosmos/cosmos-sdk)
- [libp2p](https://github.com/libp2p/specs)

---

This architecture is designed for **education and testing**. Production deployment requires:
- Full cryptographic audit
- Formal verification of consensus
- Economic security analysis
- Extensive testing and fuzzing