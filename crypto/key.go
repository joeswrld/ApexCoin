package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	
	"golang.org/x/crypto/ed25519"
	"blockchain/types"
)

// KeyPair represents a private/public key pair
type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  types.PublicKey
}

// GenerateKeyPair creates a new Ed25519 keypair
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	
	var pubKey types.PublicKey
	copy(pubKey[:], pub)
	
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  pubKey,
	}, nil
}

// WalletKeys contains view and spend keypairs for stealth addresses
type WalletKeys struct {
	ViewKeyPair  *KeyPair
	SpendKeyPair *KeyPair
}

// GenerateWalletKeys creates keys for stealth address scheme
func GenerateWalletKeys() (*WalletKeys, error) {
	viewKey, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	
	spendKey, err := GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	
	return &WalletKeys{
		ViewKeyPair:  viewKey,
		SpendKeyPair: spendKey,
	}, nil
}

// GetAddress derives the public stealth address
func (wk *WalletKeys) GetAddress() types.Address {
	return types.Address{
		ViewKey:  wk.ViewKeyPair.PublicKey,
		SpendKey: wk.SpendKeyPair.PublicKey,
	}
}

// GenerateStealthAddress creates a one-time address for a recipient
// This implements a simplified Diffie-Hellman stealth address scheme
func GenerateStealthAddress(recipientAddr types.Address) (*types.TxOutput, *KeyPair, error) {
	// Generate ephemeral keypair for this transaction
	ephemeral, err := GenerateKeyPair()
	if err != nil {
		return nil, nil, err
	}
	
	// Compute shared secret: r * A (ephemeral_priv * recipient_view_pub)
	// In real impl, use proper EC point multiplication
	// For Phase 1, we use hash-based derivation (less secure but functional)
	sharedSecret := computeSharedSecret(ephemeral.PrivateKey, recipientAddr.ViewKey)
	
	// Derive one-time spend key: P' = Hs(r*A) * G + B
	// Where B is recipient's spend public key
	oneTimeKey := deriveOneTimeKey(sharedSecret, recipientAddr.SpendKey)
	
	output := &types.TxOutput{
		StealthAddr: types.Address{
			ViewKey:  recipientAddr.ViewKey, // Keep for scanning
			SpendKey: oneTimeKey,             // One-time key
		},
		TxPublicKey: ephemeral.PublicKey, // R = r*G (public ephemeral key)
	}
	
	return output, ephemeral, nil
}

// ScanTransaction checks if a transaction output belongs to this wallet
func (wk *WalletKeys) ScanTransaction(output *types.TxOutput) (bool, *types.PublicKey, error) {
	// Compute shared secret: a * R (view_priv * tx_public_key)
	sharedSecret := computeSharedSecret(wk.ViewKeyPair.PrivateKey, output.TxPublicKey)
	
	// Derive expected one-time key
	expectedKey := deriveOneTimeKey(sharedSecret, wk.SpendKeyPair.PublicKey)
	
	// Check if it matches the output's spend key
	if expectedKey == output.StealthAddr.SpendKey {
		return true, &expectedKey, nil
	}
	
	return false, nil, nil
}

// DeriveSpendKey derives the private key to spend a stealth output
func (wk *WalletKeys) DeriveSpendKey(output *types.TxOutput) (ed25519.PrivateKey, error) {
	// Verify this output belongs to us
	belongs, _, err := wk.ScanTransaction(output)
	if err != nil {
		return nil, err
	}
	if !belongs {
		return nil, errors.New("output does not belong to this wallet")
	}
	
	// Compute shared secret
	sharedSecret := computeSharedSecret(wk.ViewKeyPair.PrivateKey, output.TxPublicKey)
	
	// Derive one-time private key: x' = Hs(r*A) + b
	// Where b is our spend private key
	// NOTE: This is simplified Ed25519 scalar addition (not cryptographically sound)
	// TODO Phase 2: Use proper edwards25519 curve operations
	
	oneTimePriv := derivePrivateKey(sharedSecret, wk.SpendKeyPair.PrivateKey)
	return oneTimePriv, nil
}

// computeSharedSecret performs ECDH (simplified for Phase 1)
func computeSharedSecret(privKey ed25519.PrivateKey, pubKey types.PublicKey) [32]byte {
	// WARNING: This is NOT proper ECDH on Ed25519
	// It's a placeholder using hash-based key derivation
	// TODO Phase 2: Use edwards25519 library for proper scalar multiplication
	
	h := sha256.New()
	h.Write(privKey[:32])
	h.Write(pubKey[:])
	return sha256.Sum256(h.Sum(nil))
}

// deriveOneTimeKey derives public one-time key from shared secret
func deriveOneTimeKey(sharedSecret [32]byte, baseKey types.PublicKey) types.PublicKey {
	// Simplified: Hash(secret || base_key)
	// Real impl: Hs(secret) * G + base_key (EC point addition)
	h := sha256.New()
	h.Write(sharedSecret[:])
	h.Write(baseKey[:])
	
	var result types.PublicKey
	copy(result[:], sha256.Sum256(h.Sum(nil))[:])
	return result
}

// derivePrivateKey derives one-time private key (simplified)
func derivePrivateKey(sharedSecret [32]byte, basePriv ed25519.PrivateKey) ed25519.PrivateKey {
	// Simplified scalar addition (NOT cryptographically correct)
	// TODO Phase 2: Use proper edwards25519 scalar operations
	h := sha256.New()
	h.Write(sharedSecret[:])
	h.Write(basePriv[:32])
	
	derived := sha256.Sum256(h.Sum(nil))
	return ed25519.PrivateKey(derived[:])
}

// GenerateKeyImage creates a unique identifier for a UTXO to prevent double-spend
func GenerateKeyImage(privKey ed25519.PrivateKey, outputKey types.PublicKey) types.PublicKey {
	// Key image: I = x * Hp(P)
	// Where x is private key, P is public key, Hp is hash-to-point
	// Simplified: Hash(priv || pub)
	
	h := sha256.New()
	h.Write(privKey[:32])
	h.Write(outputKey[:])
	
	var keyImage types.PublicKey
	copy(keyImage[:], sha256.Sum256(h.Sum(nil))[:])
	return keyImage
}