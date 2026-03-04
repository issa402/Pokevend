// ============================================================
// PokémonTool — Facebook Marketplace Scraper (Go + Playwright)
// ============================================================
// Facebook has no public API for Marketplace, so we use
// Playwright to control a real browser and extract listings.
// 
// Strategy:
//   1. Launch headless Chromium
//   2. Navigate to facebook.com/marketplace
//   3. Search for "pokemon cards"
//   4. Extract listing cards: title, price, seller, location
//   5. Publish to RabbitMQ for the notification service
// ============================================================

package scraper

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"pokemontool/scraping-service/publisher"
)

// FacebookScraper handles scraping from Facebook Marketplace
type FacebookScraper struct {
	pw        *playwright.Playwright
	publisher *publisher.Publisher
	headless  bool
	timeout   float64
}

// NewFacebookScraper creates a new Facebook Marketplace scraper
func NewFacebookScraper(pw *playwright.Playwright, pub *publisher.Publisher) *FacebookScraper {
	headless := os.Getenv("SCRAPING_HEADLESS") != "false"
	timeout  := 30.0
	if val := os.Getenv("SCRAPING_TIMEOUT_SECONDS"); val != "" {
		if t, err := strconv.ParseFloat(val, 64); err == nil {
			timeout = t
		}
	}
	return &FacebookScraper{pw: pw, publisher: pub, headless: headless, timeout: timeout}
}

// Scrape searches Facebook Marketplace and publishes discovered listings
// Returns the count of listings published and any error
func (s *FacebookScraper) Scrape(query string) (int, error) {
	// Launch a Chromium browser instance
	browser, err := s.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(s.headless),
		// Slow motion helps bypass some bot detection on first visits
		SlowMo: playwright.Float(50),
		Args: []string{
			"--no-sandbox",         // Required in Docker containers
			"--disable-dev-shm-usage",
			"--disable-blink-features=AutomationControlled", // Helps evade bot detection
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to launch browser: %w", err)
	}
	defer browser.Close()

	// Create a new browser context with a realistic user agent
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
				"AppleWebKit/537.36 (KHTML, like Gecko) " +
				"Chrome/120.0.0.0 Safari/537.36",
		),
		// Viewport mimics a standard desktop monitor
		Viewport: &playwright.Size{Width: 1280, Height: 800},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create browser context: %w", err)
	}
	defer ctx.Close()

	page, err := ctx.NewPage()
	if err != nil {
		return 0, fmt.Errorf("failed to open page: %w", err)
	}

	// Navigate to Facebook Marketplace search URL
	searchURL := fmt.Sprintf(
		"https://www.facebook.com/marketplace/search/?query=%s&exact=false",
		strings.ReplaceAll(query, " ", "+"),
	)

	log.Printf("[Facebook] Navigating to: %s", searchURL)
	if _, err = page.Goto(searchURL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(s.timeout * 1000), // Playwright uses milliseconds
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return 0, fmt.Errorf("navigation failed: %w", err)
	}

	// Wait for listing elements to appear
	// Facebook renders listings with this aria role
	if err = page.WaitForSelector(
		"[data-testid='marketplace_feed_item'], div[style*='border-radius'] a[href*='/marketplace/item']",
		playwright.PageWaitForSelectorOptions{Timeout: playwright.Float(15000)},
	); err != nil {
		log.Printf("[Facebook] Listing selector timed out — page structure may have changed")
		return 0, nil // Not a hard error — page loaded but no listings visible
	}

	// Scroll down twice to load more listings
	for i := 0; i < 2; i++ {
		page.Evaluate("window.scrollBy(0, window.innerHeight)")
		time.Sleep(1500 * time.Millisecond)
	}

	// Extract listing data using JavaScript evaluation
	// This is more resilient than CSS selectors which Facebook changes frequently
	listingsRaw, err := page.Evaluate(`
		() => {
			const links = document.querySelectorAll('a[href*="/marketplace/item/"]');
			const results = [];
			links.forEach(link => {
				const texts = link.innerText.split('\n').filter(t => t.trim());
				if (texts.length >= 2) {
					results.push({
						title:      texts[0] || 'Unknown Card',
						price:      texts.find(t => t.startsWith('$')) || '$0',
						location:   texts[texts.length - 1] || '',
						url:        link.href,
					});
				}
			});
			return results.slice(0, 50); // Cap at 50 listings per run
		}
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to extract listings: %w", err)
	}

	// Parse the JS evaluation result and publish each listing
	count := 0
	if rawSlice, ok := listingsRaw.([]interface{}); ok {
		for _, rawItem := range rawSlice {
			if item, ok := rawItem.(map[string]interface{}); ok {
				listing := buildListing(item, "facebook")
				if listing != nil {
					if err := s.publisher.Publish("scraped_listings", listing); err != nil {
						log.Printf("[Facebook] Publish error: %v", err)
					} else {
						count++
					}
				}
			}
		}
	}

	return count, nil
}

// buildListing converts the raw map from JS evaluation into a publisher.Listing
func buildListing(item map[string]interface{}, marketplace string) map[string]interface{} {
	priceStr, _ := item["price"].(string)
	price        := parsePrice(priceStr)

	cardName, _ := item["title"].(string)
	if cardName == "" {
		return nil
	}

	// Filter out non-Pokemon items by basic keyword check
	lower := strings.ToLower(cardName)
	if !strings.Contains(lower, "pokemon") && !strings.Contains(lower, "psa") &&
		!strings.Contains(lower, "charizard") && !strings.Contains(lower, "pikachu") &&
		!strings.Contains(lower, "card") && !strings.Contains(lower, "booster") {
		return nil
	}

	return map[string]interface{}{
		"cardName":    cardName,
		"price":       price,
		"marketplace": marketplace,
		"listingUrl":  item["url"],
		"location":    item["location"],
		"discoveredAt": time.Now().UTC().Format(time.RFC3339),
	}
}

// parsePrice converts "$45.00" or "$1,200" into a float64
func parsePrice(priceStr string) float64 {
	cleaned := strings.ReplaceAll(priceStr, "$", "")
	cleaned  = strings.ReplaceAll(cleaned, ",", "")
	cleaned  = strings.TrimSpace(cleaned)

	price, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return price
}
