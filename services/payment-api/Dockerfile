# ビルドのコンテキストはリポジトリのルート(Go モジュールが1つで、pkg/ を共有しているため)。
#   docker build -t pingucoin/payment-api:dev -f services/payment-api/Dockerfile .
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY services/payment-api ./services/payment-api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./services/payment-api/cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
EXPOSE 8081
ENTRYPOINT ["/api"]
