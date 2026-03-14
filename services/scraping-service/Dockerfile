# ============================================================
# PokémonTool — Go Scraping Service Dockerfile
# Uses Playwright for browser automation — needs Chromium
# ============================================================

FROM golang:1.21-bookworm AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o scraper ./main.go

FROM debian:bookworm-slim
WORKDIR /app

# Install minimal dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/scraper .

RUN useradd -m scrapeuser && chown -R scrapeuser /app
USER scrapeuser

CMD ["./scraper"]
