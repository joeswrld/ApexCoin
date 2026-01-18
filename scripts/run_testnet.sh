#!/bin/bash

set -e

echo "🚀 Privacy-PoS Blockchain Testnet Launcher"
echo "=========================================="

# Clean previous data
echo "Cleaning previous testnet data..."
rm -rf data/node1 data/node2 data/node3
mkdir -p data

# Check if validator keys exist
if [ ! -f validator1.json ] || [ ! -f validator2.json ] || [ ! -f validator3.json ]; then
    echo "❌ Validator keys not found!"
    echo "Run: ./scripts/generate_validators.sh first"
    exit 1
fi

# Build binaries
echo "Building node and wallet..."
go build -o bin/node cmd/node/main.go
go build -o bin/wallet cmd/wallet/main.go

# Start Node 1 (Bootstrap)
echo ""
echo "Starting Node 1 (Bootstrap)..."
./bin/node \
    --datadir=./data/node1 \
    --port=9001 \
    --validator=validator1.json \
    --genesis=genesis.json &
NODE1_PID=$!

sleep 3

# Get Node 1 peer ID
echo "Waiting for Node 1 to initialize..."
sleep 2

# For simplicity, we'll use a hardcoded multiaddr
# In production, parse from node1 logs
BOOTSTRAP="/ip4/127.0.0.1/tcp/9001"

# Start Node 2
echo ""
echo "Starting Node 2..."
./bin/node \
    --datadir=./data/node2 \
    --port=9002 \
    --validator=validator2.json \
    --bootstrap=$BOOTSTRAP &
NODE2_PID=$!

sleep 2

# Start Node 3
echo ""
echo "Starting Node 3..."
./bin/node \
    --datadir=./data/node3 \
    --port=9003 \
    --validator=validator3.json \
    --bootstrap=$BOOTSTRAP &
NODE3_PID=$!

echo ""
echo "✅ Testnet is running!"
echo ""
echo "Nodes:"
echo "  Node 1 (Bootstrap): PID $NODE1_PID, Port 9001"
echo "  Node 2:             PID $NODE2_PID, Port 9002"
echo "  Node 3:             PID $NODE3_PID, Port 9003"
echo ""
echo "Data directories:"
echo "  ./data/node1"
echo "  ./data/node2"
echo "  ./data/node3"
echo ""
echo "To stop the testnet:"
echo "  kill $NODE1_PID $NODE2_PID $NODE3_PID"
echo ""
echo "To send a transaction:"
echo "  ./bin/wallet send <ADDRESS> <AMOUNT>"
echo ""

# Keep script running
echo "Press Ctrl+C to stop all nodes..."
wait