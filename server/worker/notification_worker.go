// ============================================================
// FILE: server/worker/notification_worker.go
// TYPE: Background Worker — RabbitMQ Consumer + Alert Pipeline
//
// WHAT IS A BACKGROUND WORKER?
// A background worker is a goroutine that runs independently of
// the HTTP server. It's not triggered by HTTP requests — it runs
// its own event loop, processing messages from a queue.
//
// THIS WORKER'S PIPELINE:
//
//	[RabbitMQ] → consume listing message
//	[Store]    → check which users are watching that card
//	[Store]    → insert alert in PostgreSQL for matching users
//	[SSE]      → push real-time notification to browser
//
// FAANG PATTERN: Event-Driven Architecture
// Python and Go are completely decoupled via RabbitMQ.
// Python publishes "here's a new eBay listing for Charizard at $89."
// Go consumes it and decides "this is below user Alice's $100 buy target → alert!"
// Neither service calls the other directly. This means:
//   - Python can restart without affecting Go
//   - Go can restart without losing messages (messages queue up in RabbitMQ)
//   - You can scale Python and Go independently
//
// GO CONCEPTS:
//
//	goroutines, channels, amqp091-go, Ack/Nack message handling,
//	concurrent processing with go keyword, struct tags
//
// ============================================================
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"

	"pokemontool/handlers"
	"pokemontool/models"
	"pokemontool/store"
)

// Listing represents the message shape that Python publishes to RabbitMQ.
// This MUST match what Python's publisher/rabbitmq_publisher.py sends.
// If Python changes the field names, this breaks — document this contract!
//
// json tags use snake_case to match Python's convention (Python uses snake_case,
// Go uses camelCase — the JSON tags bridge this difference).
type Listing struct {
	CardName    string  `json:"card_name"`
	Price       float64 `json:"price"`
	Marketplace string  `json:"marketplace"` // "ebay", "tcgplayer", "facebook", "mercari"
	ListingURL  string  `json:"listing_url"` // direct link to the listing
}

// StartNotificationWorker starts the RabbitMQ consumer loop.
// Called as a goroutine in main.go: go worker.StartNotificationWorker(rmq, db, sseManager)
// "go" keyword means this runs concurrently alongside the HTTP server.
// The HTTP server serves requests on port 3001. This worker processes messages.
// They run simultaneously, independently, on separate goroutines.
func StartNotificationWorker(conn *amqp.Connection, db *pgxpool.Pool, mgr *handlers.SSEManager) {
	// Open a RabbitMQ channel — a logical connection inside the physical connection
	// Channels are lightweight; you can have hundreds per connection.
	ch, err := conn.Channel()
	if err != nil {
		log.Printf("[worker] channel error: %v", err)
		return
	}
	defer ch.Close()

	// QueueDeclare creates the queue if it doesn't exist, or returns existing one.
	// This is idempotent — safe to call even if queue already exists.
	// durable=true: queue survives RabbitMQ restarts (stored on disk)
	q, err := ch.QueueDeclare(
		"listings", // queue name — must match Python's publisher routing_key
		true,       // durable: survives broker restart
		false,      // auto-delete: don't delete when no consumers
		false,      // exclusive: other services can also consume
		false,      // no-wait: wait for server confirmation
		nil,        // arguments: no extra options
	)
	if err != nil {
		log.Printf("[worker] queue error: %v", err)
		return
	}

	// ch.Consume registers this worker as a consumer of the queue.
	// RabbitMQ will push messages to msg channel as they arrive.
	// autoAck=false: we manually call msg.Ack() when done (ensures no message loss)
	// If autoAck were true and our worker crashed mid-processing: message lost.
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[worker] consume error: %v", err)
		return
	}

	// Instantiate store dependencies here — worker directly uses stores, not services.
	// (Worker is infrastructure-level, not HTTP-level, so it bypasses service layer here)
	alertStore := store.NewAlertStore(db)
	priceStore := store.NewPriceAlertStore(db)
	cardStore := store.NewCardStore(db)
	watchStore := store.NewWatchlistStore(db)

	log.Println("[worker] Listening for listings on RabbitMQ...")

	// RANGE OVER CHANNEL: this loops forever, receiving one message at a time.
	// The loop blocks when there are no messages (efficient — no busy waiting).
	// It exits only if the RabbitMQ connection closes.
	for msg := range msgs {
		log.Printf("[worker] Recieved message from RabbitMq: %s", string(msg.Body))
		var listing Listing
		if err := json.Unmarshal(msg.Body, &listing); err != nil {
			log.Printf("[worker] JSON Unmarshall Error: %v", err)
			// json.Unmarshal = deserialize JSON bytes into Go struct
			// If JSON is malformed, Nack (rejected) and don't requeue (it's unprocessable)
			msg.Nack(false, false) // Nack = negative acknowledgement
			continue
		}
		// Process each listing in a goroutine so we don't block the receive loop.
		// If processListing takes 500ms, we can still receive the next message immediately.
		// GO PATTERN: "fire and forget" goroutine per message (for non-critical processing)
		log.Printf("[worker] Processing Listing: %s at $%.2f", listing.CardName, listing.Price)
		go processListing(listing, alertStore, priceStore, cardStore, watchStore, mgr)
		msg.Ack(false) // Ack = tell RabbitMQ we received and processed it (remove from queue)
	}
}

// processListing matches an incoming listing against all watchlists.
// Runs in a goroutine — concurrent with other listings being processed.
func processListing(listing Listing, alerts store.AlertStore, price store.PriceAlertStore, card store.CardStore, watch store.WatchlistStore, mgr *handlers.SSEManager) {
	ctx := context.Background()

	// Always update the general price cache first
	if err := card.UpdatePrice(ctx, listing.CardName, listing.Marketplace, listing.Price); err != nil {
		log.Printf("[worker] failed to update price : %v", err)
	}

	// ── CHECK 1: GLOBAL PRICE ALERTS ───────────────────────────
	activeSettings, err := price.GetActiveAlertsForCard(ctx, listing.CardName)
	if err == nil {
		for _, setting := range activeSettings {
			var alertType, message string

			if setting.Direction == "BELOW" && listing.Price <= setting.Threshold {
				alertType = "PRICE_DROP"
				message = fmt.Sprintf("🔥 %s dropped to $%.2f on %s (Target: $%.2f)",
					listing.CardName, listing.Price, listing.Marketplace, setting.Threshold)
			} else if setting.Direction == "ABOVE" && listing.Price >= setting.Threshold {
				alertType = "PRICE_SPIKE"
				message = fmt.Sprintf("📈 %s hit $%.2f on %s (Target: $%.2f)",
					listing.CardName, listing.Price, listing.Marketplace, setting.Threshold)
			}

			if alertType != "" {
				sendAlert(ctx, setting.UserID, alertType, message, listing, alerts, mgr)
			}
		}
	}

	// ── CHECK 2: PERSONAL WATCHLIST SNIPES ─────────────────────
	candidates, err := watch.GetAlertCandidates(ctx, listing.CardName, listing.Price)
	if err == nil {
		for _, person := range candidates {
			// Safety check: only alert if target price exists
			target := 0.0
			if person.TargetBuyPrice != nil {
				target = *person.TargetBuyPrice
			}

			message := fmt.Sprintf("🎯 Watchlist Hit: %s is $%.2f (Your Target: $%.2f)",
				listing.CardName, listing.Price, target)

			sendAlert(ctx, person.UserID, "WATCHLIST_HIT", message, listing, alerts, mgr)
		}
	}
}

// Helper function to avoid repeating the Alert + SSE logic twice
func sendAlert(ctx context.Context, userID string, aType string, msg string, l Listing, store store.AlertStore, mgr *handlers.SSEManager) {
	alert := models.Alert{
		UserID:      userID,
		CardName:    &l.CardName,
		AlertType:   aType,
		Message:     msg,
		Marketplace: &l.Marketplace,
		Price:       &l.Price,
		ListingURL:  &l.ListingURL,
	}

	alertID, err := store.Insert(ctx, alert)
	if err != nil {
		log.Printf("[worker] alert insert error: %v", err)
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type":        aType,
		"id":          alertID,
		"cardName":    l.CardName,
		"message":     msg,
		"price":       l.Price,
		"marketplace": l.Marketplace,
		"listingUrl":  l.ListingURL,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
	mgr.SendToUser(userID, string(payload))
}

// <--- This bracket ends the function

// TODO #1 (Practice): Add retry logic for failed DB inserts
// If alerts.Insert() fails (e.g., DB briefly unavailable), the alert is lost.
// Implement exponential backoff retry:
//   for attempt := 0; attempt < 3; attempt++ {
//     id, err = alerts.Insert(ctx, alert)
//     if err == nil { break }
//     time.Sleep(time.Duration(1<<attempt) * time.Second)  // 1s, 2s, 4s
//   }
// Research: "exponential backoff" — used everywhere at FAANG for resilience.

// TODO #2 (Practice): Add deduplication
// If Python publishes the same listing twice (e.g., scraper bug),
// two identical alerts would be created for the same user.
// Fix: before calling alerts.Insert(), check if an alert with the same
// (user_id, card_name, marketplace, price) was already created in the last hour.
// Add a store method: alerts.ExistsRecent(ctx, userID, cardName, marketplace, price, duration)
// This is the "idempotency" pattern — same input produces same output, no duplicates.
