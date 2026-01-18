# fix_genesis.ps1
# Extract public keys from validator JSON files and update genesis.json

Write-Host "🔧 Fixing Genesis Configuration" -ForegroundColor Cyan
Write-Host "===============================" -ForegroundColor Cyan

# Function to convert byte array to hex string
function ConvertTo-HexString {
    param([array]$bytes)
    $hex = ""
    foreach ($byte in $bytes) {
        $hex += $byte.ToString("x2")
    }
    return $hex
}

# Read validator files
$validators = @()

for ($i = 1; $i -le 3; $i++) {
    $file = "validator$i.json"
    if (-not (Test-Path $file)) {
        Write-Host "❌ $file not found!" -ForegroundColor Red
        Write-Host "Run: .\scripts\generate_validators.ps1 first" -ForegroundColor Yellow
        exit 1
    }
    
    $json = Get-Content $file | ConvertFrom-Json
    $pubKeyHex = ConvertTo-HexString -bytes $json.SpendKeyPair.PublicKey
    
    Write-Host "Validator $i public key: $pubKeyHex" -ForegroundColor Green
    
    $validators += $pubKeyHex
}

# Create genesis configuration
$genesis = @{
    chain_id = "privacy-pos-testnet"
    genesis_time = "2026-01-01T00:00:00Z"
    initial_supply = 10000000
    initial_validators = @(
        @{
            public_key = $validators[0]
            staked_amount = 100000
            active = $true
            joined_height = 0
            unbonding_until = 0
            slash_count = 0
        },
        @{
            public_key = $validators[1]
            staked_amount = 100000
            active = $true
            joined_height = 0
            unbonding_until = 0
            slash_count = 0
        },
        @{
            public_key = $validators[2]
            staked_amount = 100000
            active = $true
            joined_height = 0
            unbonding_until = 0
            slash_count = 0
        }
    )
}

# Save to genesis.json
$genesis | ConvertTo-Json -Depth 10 | Set-Content "genesis.json"

Write-Host "`n✅ genesis.json updated successfully!" -ForegroundColor Green
Write-Host "`nGenesis validators:" -ForegroundColor Cyan
for ($i = 0; $i -lt 3; $i++) {
    Write-Host "  Validator $($i+1): $($validators[$i])" -ForegroundColor White
}

Write-Host "`nYou can now start the testnet with:" -ForegroundColor Yellow
Write-Host "  .\scripts\run_testnet.ps1" -ForegroundColor White
