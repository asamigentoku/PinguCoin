param(
    [string]$Targets = "targets.txt",
    [int]$Rate = 50,
    [string]$Duration = "30s"
)
$env:Path += ";C:\vegeta"
New-Item -ItemType Directory -Force -Path results | Out-Null
$name = "results\$(Get-Date -Format yyyyMMdd-HHmmss)"

# NOTE: keep this file ASCII-only (PS 5.1 misreads BOM-less UTF-8). Use -output, not pipes (binary gets corrupted).
vegeta attack "-targets=$Targets" "-rate=$Rate" "-duration=$Duration" "-output=$name.bin"
vegeta report "$name.bin"
vegeta report "-type=hist[0,5ms,10ms,25ms,50ms,100ms,250ms,500ms,1s]" "$name.bin"
vegeta plot "-output=$name.html" "$name.bin"
Write-Host "plot: $name.html"
