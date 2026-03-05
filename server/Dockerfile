# ============================================================
# PokémonTool — Go API Gateway Dockerfile
# Multi-stage: compile in builder, run tiny Alpine binary
# Result: ~15MB image vs ~200MB for Node.js
# ============================================================

FROM golang:1.22-alpine AS builder
WORKDIR /app

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./main.go

# ---- Minimal runtime image ----
FROM alpine:3.19
WORKDIR /app

# Add CA certs for HTTPS outbound calls
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/server .

# Non-root user
RUN adduser -D -s /bin/sh appuser
USER appuser

EXPOSE 3001
CMD ["./server"]
