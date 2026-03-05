// worker/notification_worker.go — RabbitMQ consumer → alert → SSE push
// This is a background worker, not an HTTP handler.
// It consumes listings from RabbitMQ, matches against watchlists, and pushes SSE events.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/handlers"
	"pokemontool/models"
	"pokemontool/store"
)

type Listing struct {
	CardName    string  `json:"card_name"`
	Price       float64 `json:"price"`
	Marketplace string  `json:"marketplace"`
	ListingURL  string  `json:"listing_url"`
}

// StartNotificationWorker runs in a goroutine consuming the "listings" RabbitMQ queue
func StartNotificationWorker(conn *amqp.Connection, db *pgxpool.Pool, mgr *handlers.SSEManager) {
	ch, err := conn.Channel()
	if err != nil {
		log.Printf("[worker] channel error: %v", err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare("listings", true, false, false, false, nil)
	if err != nil {
		log.Printf("[worker] queue error: %v", err)
		return
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("[worker] consume error: %v", err)
		return
	}

	// Build store dependencies
	wlStore    := store.NewWatchlistStore(db)
	alertStore := store.NewAlertStore(db)

	log.Println("[worker] Listening for listings on RabbitMQ...")
	for msg := range msgs {
		var listing Listing
		if err := json.Unmarshal(msg.Body, &listing); err != nil {
			msg.Nack(false, false)
			continue
		}
		go processListing(listing, wlStore, alertStore, mgr)
		msg.Ack(false)
	}
}

func processListing(listing Listing, wl store.WatchlistStore, alerts store.AlertStore, mgr *handlers.SSEManager) {
	ctx := context.Background()

	// Find all watchlist entries matching this card name
	watched, err := wl.GetWatchedCards(ctx)
	if err != nil {
		log.Printf("[worker] watchlist query: %v", err)
		return
	}

	for _, item := range watched {
		if item.CardName != listing.CardName {
			continue
		}

		var alertType, message string
		if item.TargetBuyPrice != nil && listing.Price <= *item.TargetBuyPrice {
			alertType = "PRICE_DROP"
			message   = fmt.Sprintf("%s dropped to $%.2f on %s (target: $%.2f)",
				listing.CardName, listing.Price, listing.Marketplace, *item.TargetBuyPrice)
		} else if item.TargetSellPrice != nil && listing.Price >= *item.TargetSellPrice {
			alertType = "PRICE_SPIKE"
			message   = fmt.Sprintf("%s hit $%.2f on %s (target: $%.2f)",
				listing.CardName, listing.Price, listing.Marketplace, *item.TargetSellPrice)
		}
		if alertType == "" {
			continue
		}

		// Persist to PostgreSQL via alert store
		alert := models.Alert{
			UserID:      item.UserID,
			CardName:    &listing.CardName,
			AlertType:   alertType,
			Message:     message,
			Marketplace: &listing.Marketplace,
			Price:       &listing.Price,
			ListingURL:  &listing.ListingURL,
		}
		alertID, err := alerts.Insert(ctx, alert)
		if err != nil {
			log.Printf("[worker] alert insert: %v", err)
			continue
		}

		// Push via SSE to the user's browser
		payload, _ := json.Marshal(map[string]interface{}{
			"type":        alertType,
			"id":          alertID,
			"cardName":    listing.CardName,
			"message":     message,
			"price":       listing.Price,
			"marketplace": listing.Marketplace,
			"listingUrl":  listing.ListingURL,
			"timestamp":   time.Now().Format(time.RFC3339),
		})
		mgr.SendToUser(item.UserID, string(payload))
	}
}
