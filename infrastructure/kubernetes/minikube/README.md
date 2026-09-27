# Minikube server environment

This basic local environment runs only the three Go servers and the PostgreSQL
and Azurite services they require. The two internal APIs use plaintext gRPC
(h2c) over the private Kubernetes network. Only `pingu-api` is forwarded to
the host.

## Prerequisites

- Docker
- Minikube
- kubectl

The checked-in Secret contains development-only credentials. Replace them
before using these manifests in any shared environment.

## Start and build

Run these commands from the repository root:

```powershell
minikube start --driver=docker

minikube image build -t pingucoin/orcan-api:dev -f server/orcan-api/Dockerfile server/orcan-api
minikube image build -t pingucoin/payment-api:dev -f server/payment-api/Dockerfile server/payment-api
minikube image build -t pingucoin/pingu-api:dev -f server/pingu-api/Dockerfile server/pingu-api

kubectl apply -k infrastructure/kubernetes/minikube
kubectl wait --for=condition=ready pod --all -n pingucoin --timeout=180s
kubectl get pods -n pingucoin
```

## Access pingu-api

Keep the port-forward process running:

```powershell
kubectl port-forward -n pingucoin service/pingu-api 8082:8082
```

Open `http://localhost:8082/` for GraphQL Playground. The GraphQL endpoint is
`http://localhost:8082/api/v1/graphql`.

To inspect a gRPC server directly:

```powershell
kubectl port-forward -n pingucoin service/orcan-api 8080:8080
grpcurl -plaintext localhost:8080 list
```

Application calls require the development metadata header:

```powershell
grpcurl -plaintext -H "x-internal-token: minikube-development-token" localhost:8080 orcan.v1.ProductService/ListProducts
```

## Logs and cleanup

```powershell
kubectl logs -n pingucoin deployment/pingu-api -f
kubectl logs -n pingucoin deployment/orcan-api -f
kubectl logs -n pingucoin deployment/payment-api -f

kubectl delete -k infrastructure/kubernetes/minikube
```

PostgreSQL and Azurite use ephemeral storage in this basic configuration, so
their data is reset whenever their Pods are replaced. To rebuild an API after
a code change, run its `minikube image build` command again and restart its
deployment, for example:

```powershell
kubectl rollout restart -n pingucoin deployment/orcan-api
```
