FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/pt-dashboard .

# ──────────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /app/pt-dashboard /pt-dashboard
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8008

VOLUME ["/config"]

ENTRYPOINT ["/pt-dashboard"]
