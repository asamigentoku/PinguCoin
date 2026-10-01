# ビルドのコンテキストはリポジトリのルート(Go モジュールが1つで、pkg/ を共有しているため)。
#   docker build -t pingucoin/pingu-api:dev -f services/pingu-api/Dockerfile .
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY services/orcan-api/proto ./services/orcan-api/proto
COPY services/payment-api/proto ./services/payment-api/proto
COPY services/pingu-api ./services/pingu-api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./services/pingu-api/cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8082
ENTRYPOINT ["/api"]
