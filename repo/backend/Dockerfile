FROM golang:1.22-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

COPY go.mod ./
RUN go mod download || true

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /medops-server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget postgresql-client

WORKDIR /app

COPY --from=builder /medops-server /app/medops-server
COPY --from=builder /build/migrations /app/migrations

RUN mkdir -p /data/medops

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

CMD ["/app/medops-server"]
