// ============================================================
// FILE: services/scraping-service/scraper/mercari.go
// TYPE: Scraper — Mercari Browser Automation
//
// WHAT IS MERCARI?
// Mercari is a marketplace app popular for individual card sales.
// More willing sellers set prices below TCGplayer "market price."
// Our scraper finds these underpriced listings for deal detection.
//
// WHY MERCARI NEEDS A REAL BROWSER:
// Mercari is a React Single-Page Application (SPA).
// Like Facebook, content loads dynamically via JavaScript.
// Traditional HTTP scraping tools like requests/httpx get an empty page.
// Playwright's real Chrome browser executes the JavaScript, rendering the full DOM.
//
// DIFFERENCE FROM FACEBOOK SCRAPER:
// Facebook: uses SlowMo + specific bot evasion (Facebook is aggressive)
// Mercari: always headless, standard UserAgent, WaitUntilStateNetworkidle
//          (Mercari is less aggressive about blocking headless browsers)
// WaitUntilStateNetworkidle = waits until no network requests for 500ms
//   (guarantees React has finished rendering the listing grid)
// DomContentLoaded = faster but might catch page before React renders
//
// GO CONCEPT: Reusing buildListing() from facebook.go
// Both scrapers are in the "scraper" package.
// buildListing in facebook.go is available to mercari.go — same package = same scope.
// This is an example of PACKAGE-LEVEL code sharing in Go (less granular than OOP).
// If they were in different packages, we'd need to export it (BuildListing — capital B).
//
// SELECTORS AND FRAGILITY:
// Web scrapers break when websites redesign.
// "[data-testid]" selectors are more stable than class names (devs add testids intentionally).
// Multiple selectors separated by commas: tries first, falls back to second.
// Still fragile — expect to update these selectors over time.
//
// TODO #1 (Practice): Add login support for better access
// Mercari shows more listings to logged-in users. Add:
//   page.Fill("input[name='email']", email)
//   page.Fill("input[name='password']", password)
//   page.Click("button[type='submit']")
// Use Playwright's persistent browser context to save cookies between runs
// so you don't have to log in every time: browser.NewContext with StorageState
//
// TODO #2 (Practice): Add image URL extraction
// The current scraper captures imageUrl from <img> tags.
// After publishing to RabbitMQ, the notification_worker doesn't use image_url.
// Add image_url to the models.Alert struct and display the card image in SSE alerts.
// ============================================================

package scraper

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"

	"pokemontool/scraping-service/publisher"
)

// MercariScraper handles scraping from Mercari.com
type MercariScraper struct {
	pw        *playwright.Playwright
	publisher *publisher.Publisher
	headless  bool
}

// NewMercariScraper creates a new Mercari scraper
func NewMercariScraper(pw *playwright.Playwright, pub *publisher.Publisher) *MercariScraper {
	return &MercariScraper{
		pw:        pw,
		publisher: pub,
		headless:  true, // Always headless for Mercari
	}
}

// Scrape searches Mercari for Pokemon card listings
func (s *MercariScraper) Scrape(query string) (int, error) {
	browser, err := s.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(s.headless),
		Args:     []string{"--no-sandbox", "--disable-dev-shm-usage"},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to launch browser: %w", err)
	}
	defer browser.Close()

	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
				"AppleWebKit/537.36 (KHTML, like Gecko) " +
				"Chrome/120.0.0.0 Safari/537.36",
		),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create context: %w", err)
	}
	defer ctx.Close()

	page, err := ctx.NewPage()
	if err != nil {
		return 0, fmt.Errorf("failed to open page: %w", err)
	}

	// Mercari search URL
	searchURL := fmt.Sprintf(
		"https://www.mercari.com/search/?keyword=%s&status=on_sale",
		strings.ReplaceAll(query, " ", "+"),
	)

	log.Printf("[Mercari] Navigating to: %s", searchURL)
	if _, err = page.Goto(searchURL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return 0, fmt.Errorf("navigation failed: %w", err)
	}

	// Scroll to load more results
	page.Evaluate("window.scrollBy(0, window.innerHeight * 2)")
	time.Sleep(2 * time.Second)

	// Extract listings via JavaScript — Mercari uses React, so selectors vary
	listingsRaw, err := page.Evaluate(`
		() => {
			const items = document.querySelectorAll('[data-testid="item-cell"], li[class*="item"]');
			const results = [];
			items.forEach(item => {
				const titleEl  = item.querySelector('[class*="itemName"], [data-testid="item-name"]');
				const priceEl  = item.querySelector('[class*="itemPrice"], [data-testid="item-price"]');
				const linkEl   = item.querySelector('a[href*="/item/"]');
				const imageEl  = item.querySelector('img');
				if (titleEl && priceEl && linkEl) {
					results.push({
						title:    titleEl.innerText || '',
						price:    priceEl.innerText || '$0',
						url:      linkEl.href       || '',
						imageUrl: imageEl ? imageEl.src : '',
					});
				}
			});
			return results.slice(0, 50);
		}
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to extract listings: %w", err)
	}

	count := 0
	if rawSlice, ok := listingsRaw.([]interface{}); ok {
		for _, rawItem := range rawSlice {
			if item, ok := rawItem.(map[string]interface{}); ok {
				listing := buildListing(item, "mercari")
				if listing != nil {
					if err := s.publisher.Publish("scraped_listings", listing); err != nil {
						log.Printf("[Mercari] Publish error: %v", err)
					} else {
						count++
					}
				}
			}
		}
	}

	return count, nil
}
