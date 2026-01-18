package ledger

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
	
	"blockchain/types"
)

// State manages the UTXO set and validator states
type State struct {
	mu sync.RWMutex
	
	// UTXO set: key = hash(txhash + output_index)
	utxos map[string]*types.UTXO
	
	// Spent key images to prevent double-spend
	spentKeyImages map[types.PublicKey]bool
	
	// Validator states
	validators map[types.PublicKey]*types.ValidatorState
	
	// Current blockchain height
	height uint64
	
	// Total supply
	totalSupply uint64
}

// NewState creates a new state instance
func NewState() *State {
	return &State{
		utxos:          make(map[string]*types.UTXO),
		spentKeyImages: make(map[types.PublicKey]bool),
		validators:     make(map[types.PublicKey]*types.ValidatorState),
		height:         0,
		totalSupply:    0,
	}
}

// ApplyBlock applies a block to the state
func (s *State) ApplyBlock(block *types.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Validate block height
	if block.Header.Height != s.height+1 {
		return errors.New("invalid block height")
	}
	
	// Process each transaction
	for _, tx := range block.Transactions {
		if err := s.applyTransaction(tx, block.Header.Height); err != nil {
			return err
		}
	}
	
	// Update height
	s.height = block.Header.Height
	
	return nil
}

// applyTransaction applies a transaction to state (must hold lock)
func (s *State) applyTransaction(tx *types.Transaction, blockHeight uint64) error {
	// Verify no double-spend via key images
	for _, input := range tx.Inputs {
		if s.spentKeyImages[input.KeyImage] {
			return errors.New("double-spend detected: key image already spent")
		}
	}
	
	// Verify ring signatures
	if tx.RingSignature != nil {
		// TODO: Verify ring signature
		// For now, we assume valid
	}
	
	// Mark key images as spent
	for _, input := range tx.Inputs {
		s.spentKeyImages[input.KeyImage] = true
	}
	
	// Add new outputs to UTXO set
	txHash := tx.Hash()
	for i, output := range tx.Outputs {
		utxoKey := makeUTXOKey(txHash, uint32(i))
		
		utxo := &types.UTXO{
			TxHash:      txHash,
			OutputIndex: uint32(i),
			Output:      output,
			BlockHeight: blockHeight,
			Spent:       false,
		}
		
		s.utxos[utxoKey] = utxo
	}
	
	return nil
}

// ValidateTransaction validates a transaction against current state
func (s *State) ValidateTransaction(tx *types.Transaction) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check for double-spend
	for _, input := range tx.Inputs {
		if s.spentKeyImages[input.KeyImage] {
			return errors.New("key image already spent")
		}
	}
	
	// Verify ring signature
	if tx.RingSignature == nil {
		return errors.New("missing ring signature")
	}
	
	// Verify amounts balance (simplified - amounts are visible in Phase 1)
	var inputSum, outputSum uint64
	for _, input := range tx.Inputs {
		inputSum += input.Amount
	}
	for _, output := range tx.Outputs {
		outputSum += output.Amount
	}
	
	if inputSum != outputSum+tx.Fee {
		return errors.New("transaction amounts do not balance")
	}
	
	return nil
}

// GetUTXO retrieves a UTXO by transaction hash and output index
func (s *State) GetUTXO(txHash types.Hash, index uint32) (*types.UTXO, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	key := makeUTXOKey(txHash, index)
	utxo, exists := s.utxos[key]
	if !exists {
		return nil, errors.New("UTXO not found")
	}
	
	return utxo, nil
}

// GetAllUTXOs returns all unspent outputs (for decoy selection)
func (s *State) GetAllUTXOs() []*types.UTXO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	utxos := make([]*types.UTXO, 0, len(s.utxos))
	for _, utxo := range s.utxos {
		if !utxo.Spent {
			utxos = append(utxos, utxo)
		}
	}
	
	return utxos
}

// IsKeyImageSpent checks if a key image has been spent
func (s *State) IsKeyImageSpent(keyImage types.PublicKey) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.spentKeyImages[keyImage]
}

// AddValidator adds a new validator to the set
func (s *State) AddValidator(pubKey types.PublicKey, stake uint64, height uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.validators[pubKey]; exists {
		return errors.New("validator already exists")
	}
	
	s.validators[pubKey] = &types.ValidatorState{
		PublicKey:    pubKey,
		StakedAmount: stake,
		Active:       true,
		JoinedHeight: height,
	}
	
	return nil
}

// UpdateValidator updates validator state
func (s *State) UpdateValidator(pubKey types.PublicKey, update func(*types.ValidatorState)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	val, exists := s.validators[pubKey]
	if !exists {
		return errors.New("validator not found")
	}
	
	update(val)
	return nil
}

// GetValidator retrieves a validator's state
func (s *State) GetValidator(pubKey types.PublicKey) (*types.ValidatorState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	val, exists := s.validators[pubKey]
	if !exists {
		return nil, errors.New("validator not found")
	}
	
	return val, nil
}

// GetActiveValidators returns all active validators
func (s *State) GetActiveValidators() []*types.ValidatorState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	active := make([]*types.ValidatorState, 0)
	for _, val := range s.validators {
		if val.Active {
			active = append(active, val)
		}
	}
	
	return active
}

// ComputeStateRoot computes Merkle root of UTXO set
func (s *State) ComputeStateRoot() types.Hash {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Simplified: hash all UTXO keys
	h := sha256.New()
	
	for key := range s.utxos {
		h.Write([]byte(key))
	}
	
	return sha256.Sum256(h.Sum(nil))
}

// GetHeight returns current blockchain height
func (s *State) GetHeight() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.height
}

// makeUTXOKey creates a unique key for UTXO map
func makeUTXOKey(txHash types.Hash, index uint32) string {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, index)
	return txHash.String() + string(buf)
}

// InitializeGenesis initializes state from genesis config
func (s *State) InitializeGenesis(genesis *types.GenesisConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Add initial validators
	for _, val := range genesis.InitialValidators {
		s.validators[val.PublicKey] = &val
	}
	
	// Pre-allocate UTXOs
	// TODO: Create genesis transaction with pre-allocated outputs
	
	s.totalSupply = genesis.InitialSupply
	s.height = 0
	
	return nil
}