package types

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Hash represents a 32-byte hash
type Hash [32]byte

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// PublicKey represents an Ed25519 public key
type PublicKey [32]byte

func (pk PublicKey) String() string {
	return hex.EncodeToString(pk[:])
}

// Signature represents a cryptographic signature
type Signature [64]byte

// Address represents a stealth address
type Address struct {
	ViewKey  PublicKey // For scanning transactions
	SpendKey PublicKey // For spending outputs
}

// Block represents a finalized block in the chain
type Block struct {
	Header       BlockHeader
	Transactions []*Transaction
	Validators   []ValidatorSignature
}

// BlockHeader contains block metadata
type BlockHeader struct {
	Height        uint64
	Timestamp     int64
	PrevBlockHash Hash
	TxRoot        Hash // Merkle root of transactions
	StateRoot     Hash // UTXO set commitment
	Proposer      PublicKey
	Round         uint32 // BFT round number
}

// Hash computes the block header hash
func (bh *BlockHeader) Hash() Hash {
	data := append([]byte{}, bh.PrevBlockHash[:]...)
	data = append(data, bh.TxRoot[:]...)
	data = append(data, bh.StateRoot[:]...)
	data = append(data, bh.Proposer[:]...)
	// Add height, timestamp, round (simplified)
	return sha256.Sum256(data)
}

// ValidatorSignature represents a validator's vote on a block
type ValidatorSignature struct {
	Validator PublicKey
	Signature Signature
	Round     uint32
}

// Transaction represents a private transaction
type Transaction struct {
	Version uint8
	Inputs  []*TxInput
	Outputs []*TxOutput
	Fee     uint64 // Fee is visible (simplified)
	
	// Ring signature for sender anonymity
	RingSignature *RingSignature
	
	// Range proofs for amount hiding (placeholder for now)
	RangeProofs [][]byte
}

// TxInput references a previous output (by key image, not UTXO ID)
type TxInput struct {
	KeyImage PublicKey // Unique per output, prevents double-spend
	Amount   uint64    // Hidden in real impl, visible for Phase 1
}

// TxOutput represents a new UTXO with stealth address
type TxOutput struct {
	Amount      uint64    // Will be hidden via Pedersen commitments later
	StealthAddr Address   // One-time address
	TxPublicKey PublicKey // Ephemeral key for ECDH
}

// RingSignature provides sender anonymity
type RingSignature struct {
	Ring       []PublicKey // Set of possible signers (decoy + real)
	C          Hash        // Challenge
	Responses  []Signature // Response for each ring member
	KeyImage   PublicKey   // Unique identifier for the spent output
}

// UTXO represents an unspent transaction output
type UTXO struct {
	TxHash       Hash
	OutputIndex  uint32
	Output       *TxOutput
	BlockHeight  uint64
	Spent        bool
}

// ValidatorState tracks validator staking info
type ValidatorState struct {
	PublicKey      PublicKey
	StakedAmount   uint64
	Active         bool
	JoinedHeight   uint64
	UnbondingUntil uint64 // Block height when unbonding completes
	SlashCount     uint32
}

// StakingTx represents a special transaction for staking
type StakingTx struct {
	Type      StakingType // Bond or Unbond
	Validator PublicKey
	Amount    uint64
	Signature Signature
}

type StakingType uint8

const (
	StakingBond StakingType = iota
	StakingUnbond
)

// GenesisConfig defines initial chain state
type GenesisConfig struct {
	ChainID          string
	GenesisTime      time.Time
	InitialSupply    uint64
	InitialValidators []ValidatorState
	PreAllocations   map[Address]uint64
}

// Hash computes transaction hash
func (tx *Transaction) Hash() Hash {
	// Simplified: hash inputs + outputs
	data := []byte{}
	for _, in := range tx.Inputs {
		data = append(data, in.KeyImage[:]...)
	}
	for _, out := range tx.Outputs {
		data = append(data, out.StealthAddr.ViewKey[:]...)
		data = append(data, out.StealthAddr.SpendKey[:]...)
	}
	return sha256.Sum256(data)
}