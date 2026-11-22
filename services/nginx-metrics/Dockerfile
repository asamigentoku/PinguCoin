FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY services/nginx-metrics ./services/nginx-metrics
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/exporter ./services/nginx-metrics/cmd/exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/exporter /exporter
EXPOSE 9113
ENTRYPOINT ["/exporter"]
