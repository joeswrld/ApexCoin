package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	
	"blockchain/crypto"
	"blockchain/types"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	
	command := os.Args[1]
	
	switch command {
	case "generate":
		generateWallet()
	case "address":
		showAddress()
	case "send":
		sendTransaction()
	case "balance":
		queryBalance()
	case "stake":
		stakeTokens()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  wallet generate              - Generate new wallet keys")
	fmt.Println("  wallet address               - Show wallet address")
	fmt.Println("  wallet send <to> <amount>    - Send private transaction")
	fmt.Println("  wallet balance               - Query wallet balance")
	fmt.Println("  wallet stake <amount>        - Stake tokens as validator")
}

func generateWallet() {
	// Generate wallet keys
	wallet, err := crypto.GenerateWalletKeys()
	if err != nil {
		log.Fatalf("Failed to generate wallet: %v", err)
	}
	
	// Save to file
	data, err := json.MarshalIndent(wallet, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal wallet: %v", err)
	}
	
	filename := "wallet.json"
	if err := os.WriteFile(filename, data, 0600); err != nil {
		log.Fatalf("Failed to save wallet: %v", err)
	}
	
	// Show address
	addr := wallet.GetAddress()
	fmt.Println("Wallet generated successfully!")
	fmt.Println("Saved to:", filename)
	fmt.Println()
	fmt.Println("Your stealth address:")
	fmt.Println("  View Key: ", hex.EncodeToString(addr.ViewKey[:]))
	fmt.Println("  Spend Key:", hex.EncodeToString(addr.SpendKey[:]))
	fmt.Println()
	fmt.Println("⚠️  KEEP YOUR WALLET FILE SECURE!")
}

func showAddress() {
	wallet, err := loadWallet()
	if err != nil {
		log.Fatalf("Failed to load wallet: %v", err)
	}
	
	addr := wallet.GetAddress()
	fmt.Println("Your stealth address:")
	fmt.Println("  View Key: ", hex.EncodeToString(addr.ViewKey[:]))
	fmt.Println("  Spend Key:", hex.EncodeToString(addr.SpendKey[:]))
}

func sendTransaction() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: wallet send <recipient_address> <amount>")
		os.Exit(1)
	}
	
	recipientStr := os.Args[2]
	amountStr := os.Args[3]
	
	// Parse amount
	var amount uint64
	fmt.Sscanf(amountStr, "%d", &amount)
	
	// Parse recipient address
	recipient, err := parseAddress(recipientStr)
	if err != nil {
		log.Fatalf("Invalid recipient address: %v", err)
	}
	
	// Load wallet
	wallet, err := loadWallet()
	if err != nil {
		log.Fatalf("Failed to load wallet: %v", err)
	}
	
	// Build transaction
	tx, err := buildPrivateTransaction(wallet, recipient, amount)
	if err != nil {
		log.Fatalf("Failed to build transaction: %v", err)
	}
	
	fmt.Println("Transaction created:")
	fmt.Printf("  Amount: %d\n", amount)
	fmt.Printf("  Fee: %d\n", tx.Fee)
	fmt.Printf("  Hash: %s\n", tx.Hash())
	fmt.Println()
	fmt.Println("Broadcasting to network...")
	
	// TODO: Broadcast to network
	// For Phase 1, save to file
	txData, _ := json.MarshalIndent(tx, "", "  ")
	txFile := fmt.Sprintf("tx_%s.json", tx.Hash().String()[:8])
	os.WriteFile(txFile, txData, 0644)
	
	fmt.Printf("Transaction saved to %s\n", txFile)
	fmt.Println("Use node to broadcast this transaction")
}

func queryBalance() {
	wallet, err := loadWallet()
	if err != nil {
		log.Fatalf("Failed to load wallet: %v", err)
	}
	
	// TODO: Scan blockchain for owned outputs
	// For Phase 1, this would require connecting to a node
	
	fmt.Println("Scanning blockchain for your outputs...")
	fmt.Println()
	fmt.Println("Balance: 0 (scanning not yet implemented)")
	fmt.Println()
	fmt.Println("To check balance, you need to:")
	fmt.Println("1. Connect to a node")
	fmt.Println("2. Scan all transaction outputs")
	fmt.Println("3. Identify outputs belonging to your wallet")
	
	_ = wallet
}

func stakeTokens() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: wallet stake <amount>")
		os.Exit(1)
	}
	
	amountStr := os.Args[2]
	
	var amount uint64
	fmt.Sscanf(amountStr, "%d", &amount)
	
	wallet, err := loadWallet()
	if err != nil {
		log.Fatalf("Failed to load wallet: %v", err)
	}
	
	// Create staking transaction
	stakingTx := &types.StakingTx{
		Type:      types.StakingBond,
		Validator: wallet.SpendKeyPair.PublicKey,
		Amount:    amount,
	}
	
	// Sign staking transaction
	// TODO: Proper signature
	
	fmt.Println("Staking transaction created:")
	fmt.Printf("  Validator: %s\n", stakingTx.Validator.String())
	fmt.Printf("  Amount: %d\n", amount)
	fmt.Println()
	
	// Save to file
	data, _ := json.MarshalIndent(stakingTx, "", "  ")
	filename := "staking_tx.json"
	os.WriteFile(filename, data, 0644)
	
	fmt.Printf("Staking transaction saved to %s\n", filename)
	fmt.Println("Submit this to the network to become a validator")
}

func loadWallet() (*crypto.WalletKeys, error) {
	data, err := os.ReadFile("wallet.json")
	if err != nil {
		return nil, fmt.Errorf("wallet file not found. Run 'wallet generate' first")
	}
	
	var wallet crypto.WalletKeys
	if err := json.Unmarshal(data, &wallet); err != nil {
		return nil, err
	}
	
	return &wallet, nil
}

func parseAddress(addrStr string) (types.Address, error) {
	// Expected format: viewkey:spendkey (both hex)
	// For Phase 1 simplification
	var addr types.Address
	
	// Parse hex strings
	viewKeyHex := addrStr[:64]  // First 64 chars
	spendKeyHex := addrStr[65:] // After colon
	
	viewKey, err := hex.DecodeString(viewKeyHex)
	if err != nil {
		return addr, err
	}
	
	spendKey, err := hex.DecodeString(spendKeyHex)
	if err != nil {
		return addr, err
	}
	
	copy(addr.ViewKey[:], viewKey)
	copy(addr.SpendKey[:], spendKey)
	
	return addr, nil
}

func buildPrivateTransaction(wallet *crypto.WalletKeys, recipient types.Address, amount uint64) (*types.Transaction, error) {
	// Phase 1 simplified transaction builder
	// In production, this would:
	// 1. Scan for owned UTXOs
	// 2. Select inputs to cover amount + fee
	// 3. Create ring signature with decoys
	// 4. Generate stealth addresses for outputs
	
	// Generate stealth output for recipient
	output, ephemeral, err := crypto.GenerateStealthAddress(recipient)
	if err != nil {
		return nil, err
	}
	
	output.Amount = amount
	
	// Create change output (simplified - assume we have exact amount)
	// In production, scan for owned UTXOs and create change
	
	// Create transaction
	tx := &types.Transaction{
		Version: 1,
		Inputs:  make([]*types.TxInput, 0), // TODO: Add real inputs
		Outputs: []*types.TxOutput{output},
		Fee:     1000, // Fixed fee for Phase 1
	}
	
	// TODO: Create ring signature for inputs
	// For now, transaction is incomplete but demonstrates structure
	
	_ = ephemeral // Will be used for ECDH
	
	return tx, nil
}