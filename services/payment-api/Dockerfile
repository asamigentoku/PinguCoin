# ビルドのコンテキストはリポジトリのルート(Go モジュールが1つで、pkg/ を共有しているため)。
#   docker build -t pingucoin/payment-api:dev -f services/payment-api/Dockerfile \
#     --build-arg VERSION=v1.2.3 --build-arg COMMIT=$(git rev-parse HEAD) --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) .
# VERSION / COMMIT / BUILD_TIME は、バイナリに埋め込まれ、ログの version 属性や /version に出る(pkg/version)。省略すると dev。
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY services/payment-api ./services/payment-api
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/asamigentoku/PinguCoin/pkg/version.Version=${VERSION} -X github.com/asamigentoku/PinguCoin/pkg/version.Commit=${COMMIT} -X github.com/asamigentoku/PinguCoin/pkg/version.BuildTime=${BUILD_TIME}" \
    -o /out/api ./services/payment-api/cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.title="payment-api" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.source="https://github.com/asamigentoku/PinguCoin"
COPY --from=build /out/api /api
EXPOSE 8081
ENTRYPOINT ["/api"]
