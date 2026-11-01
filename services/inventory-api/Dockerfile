FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY pkg ./pkg
COPY services/inventory-api ./services/inventory-api
COPY services/media-api/proto ./services/media-api/proto
RUN CGO_ENABLED=0 go build -o /inventory-api ./services/inventory-api/cmd/api
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /inventory-api /inventory-api
ENTRYPOINT ["/inventory-api"]
