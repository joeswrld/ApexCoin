# Extract and convert public keys from validator files

function Convert-ArrayToHex {
    param($array)
    $hex = ""
    foreach ($byte in $array) {
        $hex += $byte.ToString("x2")
    }
    return $hex
}

for ($i = 1; $i -le 3; $i++) {
    $file = "validator$i.json"
    if (Test-Path $file) {
        $json = Get-Content $file | ConvertFrom-Json
        $pubKeyArray = $json.SpendKeyPair.PublicKey
        $hexKey = Convert-ArrayToHex -array $pubKeyArray
        Write-Host "Validator $i Public Key: $hexKey"
    }
}