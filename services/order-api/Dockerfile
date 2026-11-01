FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY services/order-api ./services/order-api
RUN CGO_ENABLED=0 go build -o /order-api ./services/order-api/cmd/api
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /order-api /order-api
ENTRYPOINT ["/order-api"]
