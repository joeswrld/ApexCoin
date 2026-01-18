package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"golang.org/x/crypto/ed25519"
	"blockchain/consensus"
	"blockchain/crypto"
	"blockchain/ledger"
	"blockchain/p2p"
	"blockchain/storage"
	"blockchain/types"
)

type Config struct {
	DataDir        string
	P2PPort        int
	BootstrapPeers []string
	ValidatorKey   string
	GenesisFile    string
}

func main() {
	// Parse flags
	cfg := parseFlags()
	
	// Initialize node
	node, err := NewNode(cfg)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}
	
	// Start node
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}
	
	log.Printf("Node started successfully")
	log.Printf("Peer ID: %s", node.network.GetHostID())
	log.Printf("Listening on: %v", node.network.GetMultiaddrs())
	
	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	log.Println("Shutting down...")
	node.Stop()
}

type Node struct {
	config    *Config
	db        *storage.Database
	state     *ledger.State
	consensus *consensus.Engine
	network   *p2p.Network
	
	// Transaction pool
	txPool []*types.Transaction
	
	// Validator identity
	validatorKey ed25519.PrivateKey
	validatorPub types.PublicKey
	isValidator  bool
}

func NewNode(cfg *Config) (*Node, error) {
	// Open database
	db, err := storage.Open(cfg.DataDir + "/blockchain.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// Initialize state
	state := ledger.NewState()
	
	// Load or create genesis
	genesis, err := loadGenesis(db, cfg.GenesisFile)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load genesis: %w", err)
	}
	
	if err := state.InitializeGenesis(genesis); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize genesis: %w", err)
	}
	
	// Load validator key if provided
	var validatorKey ed25519.PrivateKey
	var validatorPub types.PublicKey
	isValidator := false
	
	if cfg.ValidatorKey != "" {
		key, err := loadValidatorKey(cfg.ValidatorKey)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to load validator key: %w", err)
		}
		validatorKey = key.PrivateKey
		validatorPub = key.PublicKey
		isValidator = true
	}
	
	// Create consensus engine
	consensusEngine := consensus.NewEngine(state, validatorKey, validatorPub)
	if err := consensusEngine.UpdateValidatorSet(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to update validator set: %w", err)
	}
	
	// Create P2P network
	network, err := p2p.NewNetwork(cfg.P2PPort, cfg.BootstrapPeers)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create network: %w", err)
	}
	
	node := &Node{
		config:       cfg,
		db:           db,
		state:        state,
		consensus:    consensusEngine,
		network:      network,
		txPool:       make([]*types.Transaction, 0),
		validatorKey: validatorKey,
		validatorPub: validatorPub,
		isValidator:  isValidator,
	}
	
	// Set up message handlers
	network.SetBlockHandler(node.handleBlock)
	network.SetTxHandler(node.handleTransaction)
	network.SetVoteHandler(node.handleVote)
	
	return node, nil
}

func (n *Node) Start() error {
	// Start network
	if err := n.network.Start(); err != nil {
		return err
	}
	
	// Sync blockchain
	go n.syncBlockchain()
	
	// Start block production if validator
	if n.isValidator {
		go n.produceBlocks()
	}
	
	return nil
}

func (n *Node) Stop() {
	n.network.Close()
	n.db.Close()
}

func (n *Node) handleBlock(data []byte) error {
	var msg p2p.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	
	var block types.Block
	if err := json.Unmarshal(msg.Data, &block); err != nil {
		return err
	}
	
	log.Printf("Received block at height %d", block.Header.Height)
	
	// Get previous block
	prevBlock, err := n.db.GetBlock(block.Header.Height - 1)
	if err != nil {
		return fmt.Errorf("failed to get previous block: %w", err)
	}
	
	// Validate block
	if err := n.consensus.ValidateBlock(&block, prevBlock); err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}
	
	// Apply to state
	if err := n.state.ApplyBlock(&block); err != nil {
		return fmt.Errorf("failed to apply block: %w", err)
	}
	
	// Save to database
	if err := n.db.SaveBlock(&block); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}
	
	if err := n.db.UpdateLatestHeight(block.Header.Height); err != nil {
		return fmt.Errorf("failed to update height: %w", err)
	}
	
	log.Printf("Block %d finalized", block.Header.Height)
	
	return nil
}

func (n *Node) handleTransaction(data []byte) error {
	var msg p2p.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	
	var tx types.Transaction
	if err := json.Unmarshal(msg.Data, &tx); err != nil {
		return err
	}
	
	// Validate transaction
	if err := n.state.ValidateTransaction(&tx); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	
	// Add to pool
	n.txPool = append(n.txPool, &tx)
	
	log.Printf("Transaction added to pool: %s", tx.Hash())
	
	return nil
}

func (n *Node) handleVote(data []byte) error {
	var msg p2p.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	
	var vote types.ValidatorSignature
	if err := json.Unmarshal(msg.Data, &vote); err != nil {
		return err
	}
	
	// Get current block being voted on
	latestBlock, err := n.db.GetLatestBlock()
	if err != nil {
		return err
	}
	
	blockHash := latestBlock.Header.Hash()
	
	// Collect vote
	if err := n.consensus.CollectVote(&vote, blockHash); err != nil {
		return fmt.Errorf("failed to collect vote: %w", err)
	}
	
	log.Printf("Vote received from %s", vote.Validator.String()[:8])
	
	return nil
}

func (n *Node) produceBlocks() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		if err := n.proposeBlock(); err != nil {
			log.Printf("Failed to propose block: %v", err)
		}
	}
}

func (n *Node) proposeBlock() error {
	// Get current height
	height := n.state.GetHeight()
	
	// Check if we're the proposer
	proposer, err := n.consensus.SelectProposer(height+1, 0)
	if err != nil {
		return err
	}
	
	if proposer != n.validatorPub {
		return nil // Not our turn
	}
	
	// Get previous block
	prevBlock, err := n.db.GetLatestBlock()
	if err != nil {
		return err
	}
	
	// Create block with pending transactions
	txs := n.txPool
	n.txPool = make([]*types.Transaction, 0) // Clear pool
	
	block, err := n.consensus.ProposeBlock(txs, prevBlock)
	if err != nil {
		return err
	}
	
	log.Printf("Proposing block at height %d with %d transactions", block.Header.Height, len(txs))
	
	// Vote for our own block
	vote, err := n.consensus.VoteForBlock(block)
	if err != nil {
		return err
	}
	
	// Broadcast block
	if err := n.network.BroadcastBlock(block); err != nil {
		return err
	}
	
	// Broadcast our vote
	if err := n.network.BroadcastVote(vote); err != nil {
		return err
	}
	
	return nil
}

func (n *Node) syncBlockchain() {
	// TODO: Implement blockchain synchronization
	// For Phase 1, we assume genesis start
	log.Println("Blockchain sync started")
}

func parseFlags() *Config {
	dataDir := flag.String("datadir", "./data", "Data directory")
	p2pPort := flag.Int("port", 9000, "P2P listen port")
	bootstrap := flag.String("bootstrap", "", "Bootstrap peer addresses (comma-separated)")
	validatorKey := flag.String("validator", "", "Path to validator key file")
	genesisFile := flag.String("genesis", "genesis.json", "Genesis file path")
	
	flag.Parse()
	
	bootstrapPeers := []string{}
	if *bootstrap != "" {
		// Parse comma-separated peers
		// Simplified for now
		bootstrapPeers = []string{*bootstrap}
	}
	
	return &Config{
		DataDir:        *dataDir,
		P2PPort:        *p2pPort,
		BootstrapPeers: bootstrapPeers,
		ValidatorKey:   *validatorKey,
		GenesisFile:    *genesisFile,
	}
}

func loadGenesis(db *storage.Database, genesisFile string) (*types.GenesisConfig, error) {
	// Try to load from database first
	genesis, err := db.GetGenesis()
	if err == nil {
		return genesis, nil
	}
	
	// Load from file
	data, err := os.ReadFile(genesisFile)
	if err != nil {
		return nil, err
	}
	
	if err := json.Unmarshal(data, &genesis); err != nil {
		return nil, err
	}
	
	// Save to database
	if err := db.SaveGenesis(genesis); err != nil {
		return nil, err
	}
	
	return genesis, nil
}

func loadValidatorKey(path string) (*crypto.KeyPair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var keyPair crypto.KeyPair
	if err := json.Unmarshal(data, &keyPair); err != nil {
		return nil, err
	}
	
	return &keyPair, nil
}