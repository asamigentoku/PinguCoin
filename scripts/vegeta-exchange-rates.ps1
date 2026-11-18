param(
    [string]$BaseUrl = "http://localhost:8082",
    [ValidateRange(1, 100000)]
    [int]$Rate = 50,
    [string]$Duration = "30s",
    [string]$Vegeta = "vegeta",
    [string]$OutputDirectory = ".cache/vegeta-exchange-rates",
    [ValidateRange(1, 60)]
    [int]$ProgressIntervalSeconds = 5
)

$ErrorActionPreference = "Stop"

$vegetaCommand = Get-Command $Vegeta -ErrorAction SilentlyContinue
if (-not $vegetaCommand -and $Vegeta -eq "vegeta") {
    $localVegeta = Join-Path $PSScriptRoot "..\.cache\vegeta-12.13.0\vegeta.exe"
    if (Test-Path -LiteralPath $localVegeta) {
        $Vegeta = (Resolve-Path -LiteralPath $localVegeta).Path
        $vegetaCommand = Get-Command $Vegeta -ErrorAction SilentlyContinue
    }
}
if (-not $vegetaCommand) {
    throw "vegeta was not found. Download the Windows executable from https://github.com/tsenart/vegeta/releases and add vegeta.exe to PATH, or pass -Vegeta with its full path."
}
$vegetaExe = $vegetaCommand.Source
$base = $BaseUrl.TrimEnd("/") -replace '://localhost(?=[:/]|$)', '://127.0.0.1'
$runDirectory = Join-Path $OutputDirectory (Get-Date -Format "yyyyMMdd-HHmmss")
[void](New-Item -ItemType Directory -Force -Path $runDirectory)

function ConvertTo-DurationSeconds([string]$Value) {
    if ($Value -notmatch '^(?<amount>[0-9]+(?:\.[0-9]+)?)(?<unit>ms|s|m|h)$') {
        return $null
    }
    $amount = [double]$Matches.amount
    switch ($Matches.unit) {
        "ms" { return $amount / 1000 }
        "s" { return $amount }
        "m" { return $amount * 60 }
        "h" { return $amount * 3600 }
    }
}

function Invoke-VegetaComparisonTarget([string]$Name, [string]$Url, [bool]$UseCurrentSecondUrls = $false) {
    $targetPath = Join-Path $runDirectory "$Name-target.txt"
    $resultPath = Join-Path $runDirectory "$Name-results.bin"
    $reportPath = Join-Path $runDirectory "$Name-report.txt"
    $durationSeconds = ConvertTo-DurationSeconds $Duration

    if ($UseCurrentSecondUrls) {
        if ($null -eq $durationSeconds) {
            throw "v2 timestamped requests require Duration in ms, s, m, or h"
        }
        $firstSecond = [DateTime]::UtcNow
        $totalRequests = [int][math]::Round($Rate * $durationSeconds)
        $targets = [System.Text.StringBuilder]::new()
        for ($requestIndex = 0; $requestIndex -lt $totalRequests; $requestIndex++) {
            $requestTime = $firstSecond.AddSeconds([math]::Floor($requestIndex / $Rate))
            $date = $requestTime.ToString("yyyyMMdd")
            $file = $requestTime.ToString("yyyyMMddHHmmss") + ".json"
            [void]$targets.Append("GET $base/api/v2/exchange-rates/$date/$file`r`n")
        }
        [System.IO.File]::WriteAllText($targetPath, $targets.ToString(), [System.Text.Encoding]::ASCII)
        $targetDescription = "$Url/<UTC date>/<UTC second>.json"
    }
    else {
        [System.IO.File]::WriteAllText($targetPath, "GET $Url`r`n", [System.Text.Encoding]::ASCII)
        $targetDescription = $Url
    }

    Write-Host "`n=== ${Name}: $targetDescription ==="
    if ($null -ne $durationSeconds) {
        $plannedRequests = [math]::Round($Rate * $durationSeconds)
        Write-Host ("[{0:HH:mm:ss}] Starting: rate={1}/s duration={2} planned requests={3:N0}" -f (Get-Date), $Rate, $Duration, $plannedRequests)
    }
    else {
        Write-Host ("[{0:HH:mm:ss}] Starting: rate={1}/s duration={2}" -f (Get-Date), $Rate, $Duration)
    }

    $attackArguments = @("attack", "-name=$Name", "-rate=$Rate/s", "-duration=$Duration", "-targets=$targetPath", "-output=$resultPath")
    $attack = Start-Process -FilePath $vegetaExe -ArgumentList $attackArguments -NoNewWindow -PassThru
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        while (-not $attack.WaitForExit($ProgressIntervalSeconds * 1000)) {
            $elapsedSeconds = $stopwatch.Elapsed.TotalSeconds
            $estimatedRequests = [math]::Floor($Rate * $elapsedSeconds)
            if ($null -ne $durationSeconds -and $durationSeconds -gt 0) {
                $percent = [math]::Min(100, 100 * $elapsedSeconds / $durationSeconds)
                Write-Host ("[{0:HH:mm:ss}] {1} running: {2:N1}% elapsed={3:hh\:mm\:ss} estimated requests={4:N0}" -f (Get-Date), $Name, $percent, $stopwatch.Elapsed, $estimatedRequests)
            }
            else {
                Write-Host ("[{0:HH:mm:ss}] {1} running: elapsed={2:hh\:mm\:ss} estimated requests={3:N0}" -f (Get-Date), $Name, $stopwatch.Elapsed, $estimatedRequests)
            }
        }
        # Windows PowerShell 5.1 needs the parameterless wait to populate ExitCode.
        $attack.WaitForExit()
    }
    finally {
        $stopwatch.Stop()
        if (-not $attack.HasExited) {
            Stop-Process -Id $attack.Id -Force -ErrorAction SilentlyContinue
        }
    }
    # Start-Process does not reliably expose ExitCode on Windows PowerShell 5.1
    # unless it owns the wait. The result file is the portable success signal.
    $attackExitCode = $attack.ExitCode
    if ($null -ne $attackExitCode -and $attackExitCode -ne 0) {
        throw "vegeta attack failed for $Name (exit code $attackExitCode)"
    }
    if (-not (Test-Path -LiteralPath $resultPath) -or (Get-Item -LiteralPath $resultPath).Length -eq 0) {
        throw "vegeta attack failed for ${Name}: no results were written"
    }
    Write-Host ("[{0:HH:mm:ss}] {1} attack completed in {2:hh\:mm\:ss}. Generating report..." -f (Get-Date), $Name, $stopwatch.Elapsed)

    & $vegetaExe report "-type=text" "-output=$reportPath" $resultPath
    if ($LASTEXITCODE -ne 0) { throw "vegeta report failed for $Name" }
    Get-Content $reportPath | Write-Host
}

# Run separately so v1 and v2 do not steal CPU or connections from each other.
Invoke-VegetaComparisonTarget "v1-dynamic-cache" "$base/api/v1/exchange-rates"
Invoke-VegetaComparisonTarget "v2-cache" "$base/api/v2/exchange-rates" $true
Write-Host "`nResults: $runDirectory"
