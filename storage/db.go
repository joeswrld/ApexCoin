package storage

import (
	"encoding/json"
	"errors"
	
	"github.com/dgraph-io/badger/v3"
	"blockchain/types"
)

// Database wraps BadgerDB for blockchain storage
type Database struct {
	db *badger.DB
}

// Open opens or creates a BadgerDB database
func Open(path string) (*Database, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Disable logging for now
	
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	
	return &Database{db: db}, nil
}

// Close closes the database
func (d *Database) Close() error {
	return d.db.Close()
}

// SaveBlock saves a block to database
func (d *Database) SaveBlock(block *types.Block) error {
	return d.db.Update(func(txn *badger.Txn) error {
		// Serialize block
		data, err := json.Marshal(block)
		if err != nil {
			return err
		}
		
		// Save by height
		key := makeBlockKey(block.Header.Height)
		if err := txn.Set(key, data); err != nil {
			return err
		}
		
		// Save by hash
		hashKey := makeBlockHashKey(block.Header.Hash())
		return txn.Set(hashKey, data)
	})
}

// GetBlock retrieves a block by height
func (d *Database) GetBlock(height uint64) (*types.Block, error) {
	var block types.Block
	
	err := d.db.View(func(txn *badger.Txn) error {
		key := makeBlockKey(height)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &block)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &block, nil
}

// GetBlockByHash retrieves a block by hash
func (d *Database) GetBlockByHash(hash types.Hash) (*types.Block, error) {
	var block types.Block
	
	err := d.db.View(func(txn *badger.Txn) error {
		key := makeBlockHashKey(hash)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &block)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &block, nil
}

// GetLatestBlock retrieves the highest block
func (d *Database) GetLatestBlock() (*types.Block, error) {
	height, err := d.GetLatestHeight()
	if err != nil {
		return nil, err
	}
	
	return d.GetBlock(height)
}

// GetLatestHeight retrieves the latest block height
func (d *Database) GetLatestHeight() (uint64, error) {
	var height uint64
	
	err := d.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("latest_height"))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				height = 0
				return nil
			}
			return err
		}
		
		return item.Value(func(val []byte) error {
			if len(val) < 8 {
				return errors.New("invalid height data")
			}
			height = uint64(val[0]) | uint64(val[1])<<8 | uint64(val[2])<<16 | uint64(val[3])<<24 |
				uint64(val[4])<<32 | uint64(val[5])<<40 | uint64(val[6])<<48 | uint64(val[7])<<56
			return nil
		})
	})
	
	return height, err
}

// UpdateLatestHeight updates the latest block height
func (d *Database) UpdateLatestHeight(height uint64) error {
	return d.db.Update(func(txn *badger.Txn) error {
		data := make([]byte, 8)
		data[0] = byte(height)
		data[1] = byte(height >> 8)
		data[2] = byte(height >> 16)
		data[3] = byte(height >> 24)
		data[4] = byte(height >> 32)
		data[5] = byte(height >> 40)
		data[6] = byte(height >> 48)
		data[7] = byte(height >> 56)
		
		return txn.Set([]byte("latest_height"), data)
	})
}

// SaveTransaction saves a transaction
func (d *Database) SaveTransaction(tx *types.Transaction) error {
	return d.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(tx)
		if err != nil {
			return err
		}
		
		key := makeTxKey(tx.Hash())
		return txn.Set(key, data)
	})
}

// GetTransaction retrieves a transaction by hash
func (d *Database) GetTransaction(hash types.Hash) (*types.Transaction, error) {
	var tx types.Transaction
	
	err := d.db.View(func(txn *badger.Txn) error {
		key := makeTxKey(hash)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &tx)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &tx, nil
}

// SaveGenesis saves the genesis configuration
func (d *Database) SaveGenesis(genesis *types.GenesisConfig) error {
	return d.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(genesis)
		if err != nil {
			return err
		}
		
		return txn.Set([]byte("genesis"), data)
	})
}

// GetGenesis retrieves the genesis configuration
func (d *Database) GetGenesis() (*types.GenesisConfig, error) {
	var genesis types.GenesisConfig
	
	err := d.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("genesis"))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &genesis)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &genesis, nil
}

// Helper functions to create database keys
func makeBlockKey(height uint64) []byte {
	key := make([]byte, 9)
	key[0] = 'b' // block prefix
	key[1] = byte(height)
	key[2] = byte(height >> 8)
	key[3] = byte(height >> 16)
	key[4] = byte(height >> 24)
	key[5] = byte(height >> 32)
	key[6] = byte(height >> 40)
	key[7] = byte(height >> 48)
	key[8] = byte(height >> 56)
	return key
}

func makeBlockHashKey(hash types.Hash) []byte {
	key := make([]byte, 33)
	key[0] = 'h' // hash prefix
	copy(key[1:], hash[:])
	return key
}

func makeTxKey(hash types.Hash) []byte {
	key := make([]byte, 33)
	key[0] = 't' // transaction prefix
	copy(key[1:], hash[:])
	return key
}