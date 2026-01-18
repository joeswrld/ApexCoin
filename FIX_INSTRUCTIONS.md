# Genesis.json Loading Error - Fix

## Problem
The error occurred because:
```
json: unsupported type: map[types.Address]uint64
```

The `GenesisConfig` struct had a field `PreAllocations map[Address]uint64` which cannot be properly marshaled/unmarshaled by Go's JSON library because `Address` is a struct type, not a simple type like string.

Additionally, the `PublicKey` type (which is `[32]byte`) wasn't properly implementing JSON marshaling, causing issues when loading the genesis configuration.

## Solution

### 1. Fixed `types/types.go`

**Changes:**
- Removed the `PreAllocations` field from `GenesisConfig` (not needed for Phase 1)
- Added JSON struct tags to all `GenesisConfig` and `ValidatorState` fields
- Implemented `MarshalJSON` and `UnmarshalJSON` methods for `PublicKey` type to handle hex string conversion

### 2. Updated `genesis.json`

**Changes:**
- Converted public keys from byte arrays to hex strings
- Added proper JSON field names matching the struct tags
- Ensured all validator public keys are correctly formatted

### 3. Created Helper Script

**`fix_genesis.ps1`:**
- Automatically extracts public keys from validator JSON files
- Converts byte arrays to hex strings
- Generates a properly formatted genesis.json

## Files to Replace

1. **`types/types.go`** - Replace with the fixed version that includes:
   - Simplified `GenesisConfig` without `PreAllocations`
   - JSON marshaling methods for `PublicKey`
   - Proper JSON struct tags

2. **`genesis.json`** - Replace with the corrected version that has:
   - Hex-encoded public keys as strings
   - Proper JSON field names

## How to Apply the Fix

### Option 1: Manual Replacement (Recommended)

1. Replace `types/types.go` with the fixed version
2. Replace `genesis.json` with the corrected version
3. Rebuild the project:
   ```powershell
   go build -o bin\node.exe cmd\node\main.go
   ```

### Option 2: Use the Helper Script

1. Copy `fix_genesis.ps1` to your project root
2. Run it to auto-generate genesis.json:
   ```powershell
   .\fix_genesis.ps1
   ```
3. Replace `types/types.go` with the fixed version
4. Rebuild the project

## Verification

After applying the fix, you should be able to run:

```powershell
bin\node.exe --datadir=.\data\node1 --port=9001 --validator=validator1.json --genesis=genesis.json
```

Without the `unsupported type` error.

## Key Changes Summary

### Before (types.go):
```go
type GenesisConfig struct {
    ChainID          string
    GenesisTime      time.Time
    InitialSupply    uint64
    InitialValidators []ValidatorState
    PreAllocations   map[Address]uint64  // ❌ Problematic
}

type PublicKey [32]byte  // ❌ No JSON support
```

### After (types.go):
```go
type GenesisConfig struct {
    ChainID           string           `json:"chain_id"`
    GenesisTime       string           `json:"genesis_time"`
    InitialSupply     uint64           `json:"initial_supply"`
    InitialValidators []ValidatorState `json:"initial_validators"`
    // PreAllocations removed ✅
}

type PublicKey [32]byte

// MarshalJSON/UnmarshalJSON implemented ✅
```

### Before (genesis.json):
```json
{
  "initial_validators": [{
    "public_key": "a66efd268fb159c351dd9ef684b36919f784b5a59d2053f6ca98ef3dee0e",
    // ❌ Incomplete/malformed
  }]
}
```

### After (genesis.json):
```json
{
  "chain_id": "privacy-pos-testnet",
  "initial_validators": [{
    "public_key": "f095a66efd268fb159c351dd9ef684b36919f784b5a59d2053f6ca98ef3dee0e",
    // ✅ Complete 64-character hex string
  }]
}
```

## Additional Notes

- The `PreAllocations` field was removed as it's not essential for Phase 1 testnet
- If you need pre-allocated UTXOs later, you can add them through a genesis transaction
- All public keys are now properly hex-encoded strings (64 hex characters = 32 bytes)
- The fix maintains backward compatibility with existing validator JSON files
