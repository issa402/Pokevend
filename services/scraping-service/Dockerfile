# ============================================================
# PokémonTool — Go Scraping Service Dockerfile
# Uses Playwright for browser automation — needs Chromium
# ============================================================

FROM golang:1.21-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o scraper ./main.go

FROM mcr.microsoft.com/playwright/go:v0.3702.0-bookworm
WORKDIR /app
COPY --from=builder /app/scraper .

# Install Playwright browsers (Chromium only to save space)
RUN playwright install chromium --with-deps

RUN useradd -m scrapeuser && chown -R scrapeuser /app
USER scrapeuser

CMD ["./scraper"]
