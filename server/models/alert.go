// ============================================================
// FILE: server/models/alert.go
// TYPE: Models Layer — Alert Domain Type
//
// WHAT IS THIS?
// An Alert represents a fired notification.
// Created by worker/notification_worker.go when a listing triggers a threshold.
// Read by handlers/alert_handler.go for the alerts panel in the dashboard.
//
// THE ALERT LIFECYCLE:
//   1. Python publishes listing → RabbitMQ "listings" queue
//   2. Go notification_worker consumes → matches watchlists
//   3. If match → INSERT into alerts table, push via SSE
//   4. User opens dashboard → GET /api/alerts → reads from alerts table
//   5. User clicks "Mark Read" → PUT /api/alerts/{id}/read → is_read = true
//
// KEY FIELD: IsRead (bool with default false)
// The unread count in the navbar badge = COUNT(*) WHERE is_read=false
// This field drives the "notification bubble" feature.
//
// POINTER FIELDS (*string, *float64):
// Alerts from different alert_types may not have all fields.
//   TREND_CHANGE alert: has card_name, no price or listing_url
//   PRICE_DROP alert:   has card_name, price, listing_url, marketplace
// Using *string (pointer) = optional. nil means "not applicable for this alert type."
// ============================================================
package models

import "time"

// Alert represents a single notification event for a user.
type Alert struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	CardName    *string    `json:"cardName"`    // nil for non-card alerts
	AlertType   string     `json:"alertType"`   // "PRICE_DROP", "PRICE_SPIKE", "TREND_CHANGE", "DEAL_OF_DAY"
	Message     string     `json:"message"`     // human-readable: "Charizard dropped to $89 on eBay"
	Marketplace *string    `json:"marketplace"` // "ebay", "tcgplayer", nil for trends
	Price       *float64   `json:"price"`       // nil if not price-related
	ListingURL  *string    `json:"listingUrl"`  // direct link to the listing
	IsRead      bool       `json:"isRead"`      // false until user dismisses it
	CreatedAt   *time.Time `json:"createdAt"`
}

// TODO #1 (Practice): Add an AlertSummary type for the navbar badge
// The navbar shows just the unread COUNT, not all alerts.
// Instead of fetching all alerts just to count them, add:
//   type AlertSummary struct {
//     TotalUnread  int    `json:"totalUnread"`
//     LatestAlert  *Alert `json:"latestAlert"` // the most recent unread
//   }
// Add a store method: AlertStore.GetSummary(ctx, userID) (*AlertSummary, error)
// SQL: SELECT COUNT(*), MAX(created_at) FROM alerts WHERE user_id=$1 AND is_read=false
// New route: GET /api/alerts/summary — lightweight, called on every page

// TODO #2 (Practice): Add alert grouping by card
// If a user has 50 alerts for "Charizard", the alerts panel shows 50 rows.
// Add a GroupByCard() method or alert store query that returns:
//   type AlertGroup struct {
//     CardName   string  `json:"cardName"`
//     Count      int     `json:"count"`      // alerts for this card
//     LatestAt   time.Time `json:"latestAt"` // most recent alert
//     Unread     int     `json:"unread"`
//   }
// SQL: SELECT card_name, COUNT(*), MAX(created_at) FROM alerts WHERE user_id=$1 GROUP BY card_name
