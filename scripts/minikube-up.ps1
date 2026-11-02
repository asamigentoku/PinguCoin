$ErrorActionPreference = "Stop"
$workspace = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $workspace

$status = & minikube status --format='{{.Host}}' 2>$null
if ($status -ne "Running") {
    & minikube start
    if ($LASTEXITCODE -ne 0) { throw "minikube start failed" }
}

& kubectl apply -f platform/kubernetes/minikube/namespace.yaml
if ($LASTEXITCODE -ne 0) { throw "namespace creation failed" }

# Remove resources from the previous all-local profile without deleting its Blob data PVC.
& kubectl -n pingucoin delete deployment azurite --ignore-not-found
if ($LASTEXITCODE -ne 0) { throw "old Azurite deployment cleanup failed" }
& kubectl -n pingucoin delete service azurite --ignore-not-found
if ($LASTEXITCODE -ne 0) { throw "old Azurite service cleanup failed" }
& kubectl -n pingucoin delete networkpolicy allow-media-to-azurite --ignore-not-found
if ($LASTEXITCODE -ne 0) { throw "old Azurite network policy cleanup failed" }

$requiredSecrets = @("media-api-env", "payment-api-env", "bff-env")
foreach ($secretName in $requiredSecrets) {
    & kubectl -n pingucoin get secret $secretName *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "Secret '$secretName' is missing. Register each service .env manually before running this script."
    }
}

& minikube -p minikube docker-env --shell powershell | Invoke-Expression

$images = @(
    @{ Tag = "pingucoin/media-api:dev"; File = "services/media-api/Dockerfile" },
    @{ Tag = "pingucoin/inventory-api:dev"; File = "services/inventory-api/Dockerfile" },
    @{ Tag = "pingucoin/payment-api:dev"; File = "services/payment-api/Dockerfile" },
    @{ Tag = "pingucoin/order-api:dev"; File = "services/order-api/Dockerfile" },
    @{ Tag = "pingucoin/bff:dev"; File = "services/bff/Dockerfile" }
)
foreach ($image in $images) {
    & docker build -t $image.Tag -f $image.File .
    if ($LASTEXITCODE -ne 0) { throw "image build failed: $($image.Tag)" }
}

& kubectl apply -k platform/kubernetes/minikube
if ($LASTEXITCODE -ne 0) { throw "manifest apply failed" }

$deployments = @("postgres", "kafka", "media-api", "inventory-api", "payment-api", "order-api", "bff")
foreach ($deployment in $deployments) {
    & kubectl -n pingucoin rollout status "deployment/$deployment" --timeout=5m
    if ($LASTEXITCODE -ne 0) { throw "rollout failed: $deployment" }
}

Write-Host "All backend components are running in minikube."
Write-Host "To access the BFF, run:"
Write-Host "  kubectl -n pingucoin port-forward service/bff 8082:8082"
