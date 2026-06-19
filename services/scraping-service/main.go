// ============================================================
// FILE: services/scraping-service/main.go
// TYPE: Go Service Entry Point — Concurrent Web Scraper
//
// WHAT IS THIS?
// Entry point for the Go-based web scraping service.
// Uses Playwright to automate real browsers (Chromium) and scrape:
//   - Facebook Marketplace (no public API available)
//   - Mercari (heavily bot-detected, requires real browser)
//
// WHY GO FOR SCRAPING (not Python)?
// Memory efficiency: Go goroutines use ~2KB each vs Python threads at ~1MB.
// Launching 10 concurrent browser instances in Go is manageable.
// In Python with threading, you'd use much more memory.
//
// WHY PLAYWRIGHT (not requests/httpx)?
// Facebook Marketplace and Mercari use JavaScript to render content.
// A simple HTTP GET returns an empty page — content loads dynamically.
// Playwright controls a real Chrome browser that executes JavaScript,
// so you get the full rendered DOM just like a regular user would see.
//
// ARCHITECTURE:
//   main.go → goroutines → scraper/facebook.go
//                       → scraper/mercari.go
//   Both scrapers → publisher/rabbitmq_publisher.go → RabbitMQ
//
// CONCURRENT SCRAPING:
// Facebook and Mercari scrapers run simultaneously (parallel goroutines).
// sync.WaitGroup synchronizes — main goroutine waits for both to finish.
// Time savings: if each takes 2 min, parallel = 2 min total, not 4 min.
//
// GO CONCEPTS:
//   goroutines (go keyword), sync.WaitGroup, ticker, defer,
//   time.Duration, log.Fatalf, strconv.Atoi
// ============================================================
package main

import (
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"               // loads .env file
	"github.com/playwright-community/playwright-go" // browser automation

	"pokemontool/scraping-service/publisher"
	"pokemontool/scraping-service/scraper"
)

func main() {
	// ── Environment Setup ─────────────────────────────────────
	// godotenv.Load loads .env into os.Getenv() — development convenience.
	// In production (Docker): env vars are set by docker-compose.yml directly.
	// godotenv.Load returns an error if .env doesn't exist, which we log but ignore.
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file — using system environment variables")
	}

	log.Println("🕷️  PokémonTool Scraping Service starting...")

	// ── Playwright Setup ──────────────────────────────────────
	// playwright.Install() downloads browser binaries if not present.
	// The scrapers only launch Chromium, so do not download Firefox/WebKit.
	// This keeps container startup faster and avoids unnecessary network/CPU churn.
	if err := playwright.Install(&playwright.RunOptions{Browsers: []string{"chromium"}}); err != nil {
		// log.Fatalf = log.Printf + os.Exit(1) — fatal means unrecoverable
		log.Fatalf("Playwright install failed: %v", err)
	}

	// playwright.Run() starts the Playwright engine (Node.js subprocess internally)
	// Returns *playwright.Playwright which creates browsers
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("Playwright run failed: %v", err)
	}
	// defer pw.Stop() runs when main() exits — stops the Playwright subprocess.
	// GO CONCEPT: defer = guaranteed cleanup. Runs even if we return early or panic.
	// You almost ALWAYS defer Close/Stop/Cleanup right after opening a resource.
	defer pw.Stop()

	// ── RabbitMQ Publisher Setup ──────────────────────────────
	// Scraped listings are published to RabbitMQ (not written to DB directly).
	// Separation: scraper just discovers → Go notification_worker processes.
	pub, err := publisher.NewPublisher(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		log.Fatalf("RabbitMQ connection failed: %v", err)
	}
	defer pub.Close()

	// ── Scraping Interval Configuration ───────────────────────
	// Default: 30 minutes between full scraping runs.
	// Configurable via env var so ops can tune without code changes.
	// For heavier scraping, use longer intervals to avoid IP blocks.
	intervalMin := 30
	if val := os.Getenv("SCRAPING_INTERVAL_MINUTES"); val != "" {
		// strconv.Atoi converts string "30" to int 30
		// Returns (int, error) — ignore error, keep default if conversion fails
		if n, err := strconv.Atoi(val); err == nil {
			intervalMin = n
		}
	}
	log.Printf("Scraping interval: every %d minutes", intervalMin)

	// Run once immediately on startup (don't wait for first tick)
	// Important: users expect the dashboard to have data immediately after deployment
	runScrapers(pw, pub)

	// ── Ticker Loop ───────────────────────────────────────────
	// time.Ticker fires on a regular interval.
	// time.Duration(intervalMin) converts int → time.Duration (for * time.Minute)
	// PATTERN: ticker.C is a channel; "for range ticker.C" blocks until each tick
	ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
	defer ticker.Stop() // stop the ticker when main() exits

	// This loop runs forever — one iteration every intervalMin minutes
	for range ticker.C {
		runScrapers(pw, pub)
	}
}

// runScrapers launches all scrapers concurrently using goroutines.
// Each scraper runs in its own goroutine — they execute in parallel.
// sync.WaitGroup ensures we wait for ALL scrapers to finish before returning.
//
// WHY WaitGroup?
// Without it, runScrapers() would return immediately after launching goroutines.
// The main function's ticker would fire again while scrapers are still running.
// WaitGroup.Wait() blocks until all Add(n) goroutines call Done().
func runScrapers(pw *playwright.Playwright, pub *publisher.Publisher) {
	log.Println("▶ Starting scraping run...")
	start := time.Now() // capture start time to measure duration

	// sync.WaitGroup = synchronize completion of multiple goroutines
	// wg.Add(2) = "I'm about to launch 2 goroutines, wait for both"
	var wg sync.WaitGroup
	wg.Add(2)

	// ── Facebook Marketplace Scraper ──────────────────────────
	// "go func()" = launch this anonymous function as a goroutine
	// GO CONCEPT: The "go" keyword launches lightweight concurrent execution.
	// This goroutine runs concurrently with the Mercari goroutine below.
	go func() {
		// defer wg.Done() = signal "this goroutine is finished" when go func() returns.
		// ALWAYS written as the FIRST line in a goroutine — ensures it runs even on panic.
		defer wg.Done()
		fbScraper := scraper.NewFacebookScraper(pw, pub)
		count, err := fbScraper.Scrape("pokemon cards")
		if err != nil {
			log.Printf("[Facebook] Error: %v", err) // log but don't crash
		} else {
			log.Printf("[Facebook] Published %d listings", count)
		}
	}()

	// ── Mercari Scraper ───────────────────────────────────────
	// Runs IN PARALLEL with the Facebook goroutine above.
	go func() {
		defer wg.Done()
		mercariScraper := scraper.NewMercariScraper(pw, pub)
		count, err := mercariScraper.Scrape("pokemon card")
		if err != nil {
			log.Printf("[Mercari] Error: %v", err)
		} else {
			log.Printf("[Mercari] Published %d listings", count)
		}
	}()

	// BLOCK here until both goroutines call wg.Done()
	// After wg.Wait() returns, both scrapers are guaranteed to have finished.
	wg.Wait()
	log.Printf("✓ Scraping run complete in %s", time.Since(start))
	// time.Since(start) = time.Now() - start = elapsed duration
}

// TODO #1 (Practice): Add a Craigslist scraper
// Craigslist doesn't have an API, relies on JavaScript, and has many local
// Pokemon card sellers at below-market prices.
// Create scraper/craigslist.go with a CraigslistScraper struct.
// Implement Scrape(query string) (count int, err error) method.
// Add wg.Add(1) + go func() in runScrapers() for Craigslist.
// Target URL: https://www.craigslist.org/search/sss?query=pokemon+card

// TODO #2 (Practice): Add scraping success/failure metrics
// Track how many listings each scraper finds per run.
// Create a struct:
//   type ScrapeMetrics struct { Facebook int; Mercari int; LastRun time.Time }
// Update it after each run in runScrapers().
// Add a GET /metrics endpoint (simple HTTP server in main.go) that returns JSON.
// This is how FAANG monitors service health — expose metrics, graph over time.
