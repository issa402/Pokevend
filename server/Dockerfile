# ============================================================
# PokémonTool — Go API Gateway Dockerfile
# Multi-stage: compile in builder, run tiny Alpine binary
# Result: ~15MB image vs ~200MB for Node.js
# ============================================================

FROM golang:1.24-alpine AS builder
WORKDIR /app

# Copy all source code and go.mod first
COPY . .

ENV GOTOOLCHAIN=auto
# Run go mod tidy to generate go.sum and download deps
RUN go mod tidy

# Build the binary
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
