#!/bin/bash

set -e

echo "🔑 Generating Validator Keys"
echo "============================"

# Build wallet tool
echo "Building wallet tool..."
go build -o bin/wallet cmd/wallet/main.go

# Generate 3 validator wallets
for i in 1 2 3; do
    echo ""
    echo "Generating validator $i..."
    ./bin/wallet generate
    mv wallet.json validator$i.json
    echo "✅ Saved to validator$i.json"
done

echo ""
echo "✅ All validator keys generated!"
echo ""
echo "Generated files:"
echo "  - validator1.json"
echo "  - validator2.json"
echo "  - validator3.json"
echo ""
echo "⚠️  IMPORTANT: Update genesis.json with actual public keys!"
echo ""
echo "Extract public keys:"
for i in 1 2 3; do
    echo "  Validator $i:"
    # Extract spend key (validator identity)
    jq -r '.SpendKeyPair.PublicKey' validator$i.json 2>/dev/null || echo "    (Extract manually from validator$i.json)"
done

echo ""
echo "Next step: Update genesis.json, then run:"
echo "  ./scripts/run_testnet.sh"