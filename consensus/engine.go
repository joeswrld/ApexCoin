package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
	"time"
	
	"golang.org/x/crypto/ed25519"
	"blockchain/types"
	"blockchain/ledger"
)

const (
	BlockTime        = 2 * time.Second
	BFTQuorum        = 2.0 / 3.0 // 2/3 majority for finality
	UnbondingPeriod  = 100        // blocks
	SlashPercentage  = 10         // 10% of stake slashed
)

// Engine manages PoS consensus and BFT finality
type Engine struct {
	mu sync.RWMutex
	
	state        *ledger.State
	currentRound uint32
	
	// Validator set for current epoch
	validatorSet []*types.ValidatorState
	totalStake   uint64
	
	// Local validator identity (if this node is a validator)
	validatorKey ed25519.PrivateKey
	validatorPub types.PublicKey
	
	// Block proposal and voting
	pendingBlock    *types.Block
	votes           map[types.PublicKey]*types.ValidatorSignature
	proposalTimeout time.Duration
}

// NewEngine creates a new consensus engine
func NewEngine(state *ledger.State, validatorPriv ed25519.PrivateKey, validatorPub types.PublicKey) *Engine {
	return &Engine{
		state:           state,
		validatorKey:    validatorPriv,
		validatorPub:    validatorPub,
		votes:           make(map[types.PublicKey]*types.ValidatorSignature),
		proposalTimeout: BlockTime,
	}
}

// UpdateValidatorSet refreshes the validator set from state
func (e *Engine) UpdateValidatorSet() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	validators := e.state.GetActiveValidators()
	
	e.validatorSet = validators
	
	// Calculate total stake
	var total uint64
	for _, val := range validators {
		total += val.StakedAmount
	}
	e.totalStake = total
	
	return nil
}

// SelectProposer selects block proposer for current round (deterministic)
func (e *Engine) SelectProposer(height uint64, round uint32) (types.PublicKey, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if len(e.validatorSet) == 0 {
		return types.PublicKey{}, errors.New("no validators in set")
	}
	
	// Weighted random selection based on stake
	// Deterministic: Hash(height || round) mod total_stake
	seed := make([]byte, 12)
	binary.BigEndian.PutUint64(seed[0:8], height)
	binary.BigEndian.PutUint32(seed[8:12], round)
	
	hash := sha256.Sum256(seed)
	selection := binary.BigEndian.Uint64(hash[:8]) % e.totalStake
	
	// Select validator by cumulative stake
	var cumulative uint64
	for _, val := range e.validatorSet {
		cumulative += val.StakedAmount
		if selection < cumulative {
			return val.PublicKey, nil
		}
	}
	
	// Fallback to first validator (should never happen)
	return e.validatorSet[0].PublicKey, nil
}

// ProposeBlock creates a new block proposal
func (e *Engine) ProposeBlock(txs []*types.Transaction, prevBlock *types.Block) (*types.Block, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	height := prevBlock.Header.Height + 1
	
	// Compute transaction root
	txRoot := computeTxRoot(txs)
	
	// Compute state root
	stateRoot := e.state.ComputeStateRoot()
	
	header := types.BlockHeader{
		Height:        height,
		Timestamp:     time.Now().Unix(),
		PrevBlockHash: prevBlock.Header.Hash(),
		TxRoot:        txRoot,
		StateRoot:     stateRoot,
		Proposer:      e.validatorPub,
		Round:         e.currentRound,
	}
	
	block := &types.Block{
		Header:       header,
		Transactions: txs,
		Validators:   make([]types.ValidatorSignature, 0),
	}
	
	return block, nil
}

// VoteForBlock creates a validator signature for a block
func (e *Engine) VoteForBlock(block *types.Block) (*types.ValidatorSignature, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	// Verify we're a validator
	if e.validatorKey == nil {
		return nil, errors.New("not a validator")
	}
	
	// Sign block hash
	blockHash := block.Header.Hash()
	signature := ed25519.Sign(e.validatorKey, blockHash[:])
	
	var sig types.Signature
	copy(sig[:], signature)
	
	vote := &types.ValidatorSignature{
		Validator: e.validatorPub,
		Signature: sig,
		Round:     e.currentRound,
	}
	
	return vote, nil
}

// CollectVote adds a validator vote to the pending block
func (e *Engine) CollectVote(vote *types.ValidatorSignature, blockHash types.Hash) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Verify validator is in set
	validator, err := e.state.GetValidator(vote.Validator)
	if err != nil {
		return errors.New("unknown validator")
	}
	
	if !validator.Active {
		return errors.New("inactive validator")
	}
	
	// Verify signature
	pubKey := ed25519.PublicKey(vote.Validator[:])
	valid := ed25519.Verify(pubKey, blockHash[:], vote.Signature[:])
	if !valid {
		return errors.New("invalid signature")
	}
	
	// Check for double-voting (slashing condition)
	if existing, exists := e.votes[vote.Validator]; exists {
		if existing.Round == vote.Round {
			// Double vote detected - slash validator
			e.slashValidator(vote.Validator, "double-vote")
			return errors.New("double-vote detected")
		}
	}
	
	// Store vote
	e.votes[vote.Validator] = vote
	
	return nil
}

// HasQuorum checks if we have 2/3+ validator votes
func (e *Engine) HasQuorum() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	var voteStake uint64
	for validator := range e.votes {
		val, err := e.state.GetValidator(validator)
		if err != nil {
			continue
		}
		voteStake += val.StakedAmount
	}
	
	quorumThreshold := uint64(float64(e.totalStake) * BFTQuorum)
	return voteStake >= quorumThreshold
}

// FinalizeBlock finalizes a block with validator signatures
func (e *Engine) FinalizeBlock(block *types.Block) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Add all votes to block
	for _, vote := range e.votes {
		block.Validators = append(block.Validators, *vote)
	}
	
	// Verify quorum
	if !e.HasQuorum() {
		return errors.New("insufficient validator votes for finality")
	}
	
	// Clear votes for next round
	e.votes = make(map[types.PublicKey]*types.ValidatorSignature)
	e.currentRound++
	
	return nil
}

// slashValidator penalizes a validator for misbehavior
func (e *Engine) slashValidator(validator types.PublicKey, reason string) {
	err := e.state.UpdateValidator(validator, func(val *types.ValidatorState) {
		// Slash stake
		slashAmount := val.StakedAmount * SlashPercentage / 100
		val.StakedAmount -= slashAmount
		
		// Increment slash count
		val.SlashCount++
		
		// Deactivate if slashed too many times
		if val.SlashCount >= 3 {
			val.Active = false
		}
	})
	
	if err != nil {
		// Log error (in real impl)
		return
	}
}

// ValidateBlock validates a proposed block
func (e *Engine) ValidateBlock(block *types.Block, prevBlock *types.Block) error {
	// Validate height
	if block.Header.Height != prevBlock.Header.Height+1 {
		return errors.New("invalid block height")
	}
	
	// Validate previous block hash
	if block.Header.PrevBlockHash != prevBlock.Header.Hash() {
		return errors.New("invalid previous block hash")
	}
	
	// Validate timestamp (not too far in future)
	now := time.Now().Unix()
	if block.Header.Timestamp > now+60 {
		return errors.New("block timestamp too far in future")
	}
	
	// Validate proposer
	proposer, err := e.SelectProposer(block.Header.Height, block.Header.Round)
	if err != nil {
		return err
	}
	
	if proposer != block.Header.Proposer {
		return errors.New("invalid proposer for this round")
	}
	
	// Validate transactions
	for _, tx := range block.Transactions {
		if err := e.state.ValidateTransaction(tx); err != nil {
			return err
		}
	}
	
	return nil
}

// computeTxRoot computes Merkle root of transactions (simplified)
func computeTxRoot(txs []*types.Transaction) types.Hash {
	h := sha256.New()
	
	for _, tx := range txs {
		txHash := tx.Hash()
		h.Write(txHash[:])
	}
	
	return sha256.Sum256(h.Sum(nil))
}

// ProcessStakingTx processes a staking transaction
func (e *Engine) ProcessStakingTx(stx *types.StakingTx, height uint64) error {
	switch stx.Type {
	case types.StakingBond:
		// Add validator
		return e.state.AddValidator(stx.Validator, stx.Amount, height)
		
	case types.StakingUnbond:
		// Mark for unbonding
		return e.state.UpdateValidator(stx.Validator, func(val *types.ValidatorState) {
			val.Active = false
			val.UnbondingUntil = height + UnbondingPeriod
		})
		
	default:
		return errors.New("unknown staking type")
	}
}