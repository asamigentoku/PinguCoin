param(
    [string]$BaseUrl = "http://localhost:8082",
    [ValidateRange(1, 1000)]
    [int]$Requests = 10,
    [ValidateRange(0, 100)]
    [int]$Warmup = 2
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http
$base = $BaseUrl.TrimEnd("/")
$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromSeconds(30)

function Invoke-ExchangeRequest([string]$Url) {
    $watch = [System.Diagnostics.Stopwatch]::StartNew()
    $response = $client.GetAsync($Url).GetAwaiter().GetResult()
    $body = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    $watch.Stop()
    if (-not $response.IsSuccessStatusCode) {
        $status = [int]$response.StatusCode
        $response.Dispose()
        throw "GET $Url returned HTTP $status`: $body"
    }
    $cacheStatus = ""
    if ($response.Headers.Contains("X-Cache-Status")) {
        $cacheStatus = ($response.Headers.GetValues("X-Cache-Status") | Select-Object -First 1)
    }
    $response.Dispose()
    return [pscustomobject]@{ Milliseconds = $watch.Elapsed.TotalMilliseconds; Body = $body; CacheStatus = $cacheStatus }
}

function Get-Percentile([double[]]$Values, [double]$Percentile) {
    $sorted = @($Values | Sort-Object)
    $index = [Math]::Ceiling($Percentile * $sorted.Count) - 1
    return $sorted[[Math]::Max(0, $index)]
}

function Get-LatencySummary([string]$Version, [double[]]$Values) {
    return [pscustomobject]@{
        Version = $Version
        Requests = $Values.Count
        AverageMs = [Math]::Round(($Values | Measure-Object -Average).Average, 2)
        P50Ms = [Math]::Round((Get-Percentile $Values 0.50), 2)
        P95Ms = [Math]::Round((Get-Percentile $Values 0.95), 2)
    }
}

function Get-CurrentV2Url {
    $now = [DateTime]::UtcNow
    return "$base/api/v2/exchange-rates/$($now.ToString('yyyyMMdd'))/$($now.ToString('yyyyMMddHHmmss')).json"
}

try {
    for ($i = 0; $i -lt $Warmup; $i++) {
        [void](Invoke-ExchangeRequest "$base/api/v1/exchange-rates")
        [void](Invoke-ExchangeRequest (Get-CurrentV2Url))
    }

    $v1 = [System.Collections.Generic.List[double]]::new()
    $v2 = [System.Collections.Generic.List[double]]::new()
    $v1CacheStatuses = [System.Collections.Generic.List[string]]::new()
    $v2CacheStatuses = [System.Collections.Generic.List[string]]::new()

    for ($i = 0; $i -lt $Requests; $i++) {
        $live = Invoke-ExchangeRequest "$base/api/v1/exchange-rates"
        $v1.Add($live.Milliseconds)
        $v1CacheStatuses.Add($live.CacheStatus)

        $snapshot = Invoke-ExchangeRequest (Get-CurrentV2Url)
        $v2.Add($snapshot.Milliseconds)
        $v2CacheStatuses.Add($snapshot.CacheStatus)
    }

    Get-LatencySummary "v1 cached dynamic BFF + media-api" $v1.ToArray()
    Get-LatencySummary "v2 cached timestamped file" $v2.ToArray()

    foreach ($result in @(
        [pscustomobject]@{ Name = "v1"; Statuses = $v1CacheStatuses },
        [pscustomobject]@{ Name = "v2"; Statuses = $v2CacheStatuses }
    )) {
        $cacheable = @($result.Statuses | Where-Object { $_ -ne "" })
        $hits = @($cacheable | Where-Object { $_ -eq "HIT" }).Count
        $hitRate = if ($cacheable.Count -eq 0) { 0 } else { 100 * $hits / $cacheable.Count }
        "{0} nginx cache hit rate: {1:N2}% ({2}/{3})" -f $result.Name, $hitRate, $hits, $cacheable.Count
        $result.Statuses | Group-Object | Sort-Object Name | ForEach-Object { "  {0}: {1}" -f $_.Name, $_.Count }
    }
}
finally {
    $client.Dispose()
}
