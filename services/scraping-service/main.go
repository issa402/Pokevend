// ============================================================
// PokémonTool — Go Scraping Service Entry Point
// Runs concurrent Playwright scrapers for Facebook Marketplace
// and Mercari to find Pokemon card listings without an official API.
// Publishes discovered listings to RabbitMQ.
// ============================================================

package main

import (
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/playwright-community/playwright-go"

	"pokemontool/scraping-service/publisher"
	"pokemontool/scraping-service/scraper"
)

func main() {
	// Load .env file if present (development mode)
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found — using environment variables directly")
	}

	log.Println("🕷️  PokémonTool Scraping Service starting...")

	// Install Playwright browsers if not already present
	// This needs to run once; in Docker it's handled by the Dockerfile
	if err := playwright.Install(); err != nil {
		log.Fatalf("Failed to install Playwright browsers: %v", err)
	}

	// Initialize Playwright engine
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("Failed to start Playwright: %v", err)
	}
	defer pw.Stop()

	// Initialize RabbitMQ publisher
	pub, err := publisher.NewPublisher(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer pub.Close()

	// Read poll interval from environment (default: 30 minutes)
	intervalMin := 30
	if val := os.Getenv("SCRAPING_INTERVAL_MINUTES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			intervalMin = n
		}
	}

	log.Printf("Scraping interval: every %d minutes", intervalMin)

	// Run immediately on startup, then on the configured schedule
	runScrapers(pw, pub)

	ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		runScrapers(pw, pub)
	}
}

// runScrapers launches all scraper goroutines in parallel.
// Go's goroutines make concurrent browser instances very efficient.
func runScrapers(pw *playwright.Playwright, pub *publisher.Publisher) {
	log.Println("Starting scraping run...")
	start := time.Now()

	var wg sync.WaitGroup // WaitGroup waits for all goroutines to finish

	// Launch Facebook Marketplace scraper in its own goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		fbScraper := scraper.NewFacebookScraper(pw, pub)
		count, err := fbScraper.Scrape("pokemon cards")
		if err != nil {
			log.Printf("[Facebook] Scrape error: %v", err)
		} else {
			log.Printf("[Facebook] Published %d listings", count)
		}
	}()

	// Launch Mercari scraper in its own goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		mercariScraper := scraper.NewMercariScraper(pw, pub)
		count, err := mercariScraper.Scrape("pokemon card")
		if err != nil {
			log.Printf("[Mercari] Scrape error: %v", err)
		} else {
			log.Printf("[Mercari] Published %d listings", count)
		}
	}()

	wg.Wait() // Block until both scrapers finish
	log.Printf("Scraping run complete in %s", time.Since(start))
}
