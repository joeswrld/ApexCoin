# run_testnet.ps1
# Windows PowerShell script to run a 3-node testnet

Write-Host "🚀 Privacy-PoS Blockchain Testnet Launcher" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# Clean previous data
Write-Host "`nCleaning previous testnet data..." -ForegroundColor Yellow
if (Test-Path "data") {
    Remove-Item -Recurse -Force data\node1, data\node2, data\node3 -ErrorAction SilentlyContinue
}
New-Item -ItemType Directory -Force -Path data | Out-Null

# Check if validator keys exist
if (-not (Test-Path "validator1.json") -or -not (Test-Path "validator2.json") -or -not (Test-Path "validator3.json")) {
    Write-Host "❌ Validator keys not found!" -ForegroundColor Red
    Write-Host "Run: .\generate_validators.ps1 first" -ForegroundColor Yellow
    exit 1
}

# Build binaries
Write-Host "`nBuilding node and wallet..." -ForegroundColor Yellow
go build -o bin\node.exe cmd\node\main.go
go build -o bin\wallet.exe cmd\wallet\main.go

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Build failed!" -ForegroundColor Red
    exit 1
}

# Start Node 1 (Bootstrap)
Write-Host "`nStarting Node 1 (Bootstrap)..." -ForegroundColor Green
$node1 = Start-Process -FilePath ".\bin\node.exe" -ArgumentList "--datadir=.\data\node1", "--port=9001", "--validator=validator1.json", "--genesis=genesis.json" -PassThru -WindowStyle Normal
Start-Sleep -Seconds 3

# Bootstrap address (simplified - Node 1 on localhost)
$bootstrap = "/ip4/127.0.0.1/tcp/9001"

# Start Node 2
Write-Host "`nStarting Node 2..." -ForegroundColor Green
$node2 = Start-Process -FilePath ".\bin\node.exe" -ArgumentList "--datadir=.\data\node2", "--port=9002", "--validator=validator2.json", "--bootstrap=$bootstrap" -PassThru -WindowStyle Normal
Start-Sleep -Seconds 2

# Start Node 3
Write-Host "`nStarting Node 3..." -ForegroundColor Green
$node3 = Start-Process -FilePath ".\bin\node.exe" -ArgumentList "--datadir=.\data\node3", "--port=9003", "--validator=validator3.json", "--bootstrap=$bootstrap" -PassThru -WindowStyle Normal

Write-Host "`n✅ Testnet is running!" -ForegroundColor Green
Write-Host "`nNodes:" -ForegroundColor Cyan
Write-Host "  Node 1 (Bootstrap): PID $($node1.Id), Port 9001" -ForegroundColor White
Write-Host "  Node 2:             PID $($node2.Id), Port 9002" -ForegroundColor White
Write-Host "  Node 3:             PID $($node3.Id), Port 9003" -ForegroundColor White

Write-Host "`nData directories:" -ForegroundColor Cyan
Write-Host "  .\data\node1"
Write-Host "  .\data\node2"
Write-Host "  .\data\node3"

Write-Host "`nTo stop the testnet:" -ForegroundColor Yellow
Write-Host "  Stop-Process -Id $($node1.Id), $($node2.Id), $($node3.Id)" -ForegroundColor White

Write-Host "`nTo send a transaction:" -ForegroundColor Yellow
Write-Host "  .\bin\wallet.exe send <ADDRESS> <AMOUNT>" -ForegroundColor White

Write-Host "`nPress Ctrl+C to stop monitoring. Nodes will continue running in background." -ForegroundColor Gray
Write-Host "Use Task Manager or the Stop-Process command above to stop nodes.`n" -ForegroundColor Gray

# Keep script running and monitor processes
try {
    while ($true) {
        Start-Sleep -Seconds 5
        if (-not (Get-Process -Id $node1.Id -ErrorAction SilentlyContinue)) {
            Write-Host "⚠️  Node 1 stopped!" -ForegroundColor Red
            break
        }
        if (-not (Get-Process -Id $node2.Id -ErrorAction SilentlyContinue)) {
            Write-Host "⚠️  Node 2 stopped!" -ForegroundColor Red
            break
        }
        if (-not (Get-Process -Id $node3.Id -ErrorAction SilentlyContinue)) {
            Write-Host "⚠️  Node 3 stopped!" -ForegroundColor Red
            break
        }
    }
} catch {
    Write-Host "`n⚠️  Monitoring stopped. Nodes are still running." -ForegroundColor Yellow
    Write-Host "To stop: Stop-Process -Id $($node1.Id), $($node2.Id), $($node3.Id)" -ForegroundColor White
}