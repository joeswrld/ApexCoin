# generate_validators.ps1
# Windows PowerShell script to generate validator keys

Write-Host "🔑 Generating Validator Keys" -ForegroundColor Cyan
Write-Host "============================" -ForegroundColor Cyan

# Build wallet tool
Write-Host "`nBuilding wallet tool..." -ForegroundColor Yellow
go build -o bin\wallet.exe cmd\wallet\main.go

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to build wallet" -ForegroundColor Red
    exit 1
}

# Generate 3 validator wallets
for ($i = 1; $i -le 3; $i++) {
    Write-Host "`nGenerating validator $i..." -ForegroundColor Yellow
    .\bin\wallet.exe generate
    Move-Item -Force wallet.json "validator$i.json"
    Write-Host "✅ Saved to validator$i.json" -ForegroundColor Green
}

Write-Host "`n✅ All validator keys generated!" -ForegroundColor Green
Write-Host "`nGenerated files:" -ForegroundColor Cyan
Write-Host "  - validator1.json"
Write-Host "  - validator2.json"
Write-Host "  - validator3.json"

Write-Host "`n⚠️  IMPORTANT: Update genesis.json with actual public keys!" -ForegroundColor Yellow
Write-Host "`nExtract public keys:" -ForegroundColor Cyan

for ($i = 1; $i -le 3; $i++) {
    Write-Host "  Validator $i:" -ForegroundColor White
    if (Get-Command jq -ErrorAction SilentlyContinue) {
        $pubkey = (Get-Content "validator$i.json" | jq -r '.SpendKeyPair.PublicKey')
        Write-Host "    $pubkey" -ForegroundColor Gray
    } else {
        Write-Host "    (Extract manually from validator$i.json)" -ForegroundColor Gray
    }
}

Write-Host "`nNext step: Update genesis.json, then run:" -ForegroundColor Cyan
Write-Host "  .\run_testnet.ps1" -ForegroundColor White