package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	
	"blockchain/types"
)

const (
	ProtocolID    = "/blockchain/1.0.0"
	BlockTopic    = "blocks"
	TxTopic       = "transactions"
	VoteTopic     = "votes"
	MaxPeers      = 50
	PeerTimeout   = 30 * time.Second
)

// Network manages P2P communication
type Network struct {
	host      host.Host
	pubsub    *pubsub.PubSub
	ctx       context.Context
	cancel    context.CancelFunc
	
	// Topic subscriptions
	blockSub *pubsub.Subscription
	txSub    *pubsub.Subscription
	voteSub  *pubsub.Subscription
	
	// Message handlers
	blockHandler MessageHandler
	txHandler    MessageHandler
	voteHandler  MessageHandler
	
	// Peer management
	peers     map[peer.ID]time.Time
	peerMutex sync.RWMutex
}

// MessageHandler processes incoming messages
type MessageHandler func(data []byte) error

// Message types
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// NewNetwork creates a new P2P network node
func NewNetwork(listenPort int, bootstrapPeers []string) (*Network, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort),
		),
	)
	if err != nil {
		cancel()
		return nil, err
	}
	
	// Create pubsub instance
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		cancel()
		h.Close()
		return nil, err
	}
	
	n := &Network{
		host:   h,
		pubsub: ps,
		ctx:    ctx,
		cancel: cancel,
		peers:  make(map[peer.ID]time.Time),
	}
	
	// Connect to bootstrap peers
	for _, addr := range bootstrapPeers {
		if err := n.connectPeer(addr); err != nil {
			fmt.Printf("Failed to connect to bootstrap peer %s: %v\n", addr, err)
		}
	}
	
	return n, nil
}

// Start starts the network services
func (n *Network) Start() error {
	// Subscribe to topics
	blockSub, err := n.pubsub.Subscribe(BlockTopic)
	if err != nil {
		return err
	}
	n.blockSub = blockSub
	
	txSub, err := n.pubsub.Subscribe(TxTopic)
	if err != nil {
		return err
	}
	n.txSub = txSub
	
	voteSub, err := n.pubsub.Subscribe(VoteTopic)
	if err != nil {
		return err
	}
	n.voteSub = voteSub
	
	// Start message listeners
	go n.handleMessages(blockSub, n.blockHandler)
	go n.handleMessages(txSub, n.txHandler)
	go n.handleMessages(voteSub, n.voteHandler)
	
	// Start peer management
	go n.managePeers()
	
	return nil
}

// SetBlockHandler sets the handler for block messages
func (n *Network) SetBlockHandler(handler MessageHandler) {
	n.blockHandler = handler
}

// SetTxHandler sets the handler for transaction messages
func (n *Network) SetTxHandler(handler MessageHandler) {
	n.txHandler = handler
}

// SetVoteHandler sets the handler for vote messages
func (n *Network) SetVoteHandler(handler MessageHandler) {
	n.voteHandler = handler
}

// BroadcastBlock broadcasts a block to the network
func (n *Network) BroadcastBlock(block *types.Block) error {
	data, err := json.Marshal(block)
	if err != nil {
		return err
	}
	
	msg := Message{
		Type: "block",
		Data: data,
	}
	
	return n.publish(BlockTopic, msg)
}

// BroadcastTransaction broadcasts a transaction to the network
func (n *Network) BroadcastTransaction(tx *types.Transaction) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	
	msg := Message{
		Type: "transaction",
		Data: data,
	}
	
	return n.publish(TxTopic, msg)
}

// BroadcastVote broadcasts a validator vote to the network
func (n *Network) BroadcastVote(vote *types.ValidatorSignature) error {
	data, err := json.Marshal(vote)
	if err != nil {
		return err
	}
	
	msg := Message{
		Type: "vote",
		Data: data,
	}
	
	return n.publish(VoteTopic, msg)
}

// publish publishes a message to a topic
func (n *Network) publish(topic string, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	return n.pubsub.Publish(topic, data)
}

// handleMessages listens for messages on a subscription
func (n *Network) handleMessages(sub *pubsub.Subscription, handler MessageHandler) {
	for {
		msg, err := sub.Next(n.ctx)
		if err != nil {
			if n.ctx.Err() != nil {
				return // Context cancelled
			}
			fmt.Printf("Error receiving message: %v\n", err)
			continue
		}
		
		// Skip messages from self
		if msg.ReceivedFrom == n.host.ID() {
			continue
		}
		
		// Update peer activity
		n.updatePeer(msg.ReceivedFrom)
		
		// Handle message
		if handler != nil {
			if err := handler(msg.Data); err != nil {
				fmt.Printf("Error handling message: %v\n", err)
			}
		}
	}
}

// connectPeer connects to a peer
func (n *Network) connectPeer(addrStr string) error {
	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		return err
	}
	
	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return err
	}
	
	return n.host.Connect(n.ctx, *peerInfo)
}

// updatePeer updates peer's last seen time
func (n *Network) updatePeer(p peer.ID) {
	n.peerMutex.Lock()
	defer n.peerMutex.Unlock()
	
	n.peers[p] = time.Now()
}

// managePeers periodically cleans up inactive peers
func (n *Network) managePeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			n.cleanupPeers()
		case <-n.ctx.Done():
			return
		}
	}
}

// cleanupPeers removes inactive peers
func (n *Network) cleanupPeers() {
	n.peerMutex.Lock()
	defer n.peerMutex.Unlock()
	
	now := time.Now()
	for p, lastSeen := range n.peers {
		if now.Sub(lastSeen) > PeerTimeout {
			delete(n.peers, p)
			n.host.Network().ClosePeer(p)
		}
	}
}

// GetPeerCount returns the number of connected peers
func (n *Network) GetPeerCount() int {
	n.peerMutex.RLock()
	defer n.peerMutex.RUnlock()
	
	return len(n.peers)
}

// GetHostID returns this node's peer ID
func (n *Network) GetHostID() peer.ID {
	return n.host.ID()
}

// GetMultiaddrs returns this node's listen addresses
func (n *Network) GetMultiaddrs() []multiaddr.Multiaddr {
	return n.host.Addrs()
}

// Close shuts down the network
func (n *Network) Close() error {
	n.cancel()
	return n.host.Close()
}