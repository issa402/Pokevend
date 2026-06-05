# ============================================================
# PokémonTool — Go Scraping Service Dockerfile
# Uses Playwright for browser automation — needs Chromium
# ============================================================

FROM golang:1.21-bookworm AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./main.go

FROM debian:bookworm-slim
WORKDIR /app

# Install minimal dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libcairo2 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libgbm1 \
    libglib2.0-0 \
    libnspr4 \
    libnss3 \
    libpango-1.0-0 \
    libx11-6 \
    libxcb1 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxkbcommon0 \
    libxrandr2 \
    wget \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/main .

RUN useradd -m scrapeuser && chown -R scrapeuser /app
USER scrapeuser

CMD ["./main"]
