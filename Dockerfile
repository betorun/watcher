FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o watcher ./cmd/server

FROM alpine:3.23.7
RUN apk --no-cache add ca-certificates
# Hors de /root : le chart tourne sous l'UID 65532 (runAsNonRoot).
COPY --from=builder /app/watcher /usr/local/bin/watcher
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/watcher"]
