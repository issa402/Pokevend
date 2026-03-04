// ============================================================
// PokémonTool — RabbitMQ Publisher (Go)
// Publishes scraped listing data to the message queue.
// The Node.js notification service consumes these messages.
// ============================================================

package publisher

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher manages a persistent RabbitMQ connection and channel
type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	url     string
}

// NewPublisher creates a new RabbitMQ publisher with retry logic
func NewPublisher(url string) (*Publisher, error) {
	if url == "" {
		url = "amqp://guest:guest@localhost:5672"
	}

	p := &Publisher{url: url}
	if err := p.connect(); err != nil {
		return nil, err
	}
	return p, nil
}

// connect establishes the AMQP connection (called on init and reconnect)
func (p *Publisher) connect() error {
	var err error
	// Retry connection up to 10 times with exponential backoff
	for i := 0; i < 10; i++ {
		p.conn, err = amqp.Dial(p.url)
		if err == nil {
			break
		}
		wait := time.Duration(1<<uint(i)) * time.Second // 1s, 2s, 4s, 8s...
		if wait > 30*time.Second {
			wait = 30 * time.Second // Cap at 30 seconds
		}
		log.Printf("[Publisher] RabbitMQ connect failed (%d/10): %v. Retrying in %s...", i+1, err, wait)
		time.Sleep(wait)
	}
	if err != nil {
		return fmt.Errorf("could not connect to RabbitMQ after 10 attempts: %w", err)
	}

	// Create a channel for publishing
	p.channel, err = p.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	// Declare required queues (idempotent — safe to call on reconnect)
	for _, queue := range []string{"scraped_listings", "listings"} {
		_, err = p.channel.QueueDeclare(
			queue, // Queue name
			true,  // Durable — survives broker restarts
			false, // Auto-delete when unused
			false, // Non-exclusive
			false, // No-wait
			nil,   // No extra arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue, err)
		}
	}

	log.Println("[Publisher] ✓ Connected to RabbitMQ")
	return nil
}

// Publish sends a listing map to the specified queue as JSON.
// Automatically reconnects if the connection was dropped.
func (p *Publisher) Publish(queue string, listing map[string]interface{}) error {
	// Reconnect if needed
	if p.conn == nil || p.conn.IsClosed() {
		log.Println("[Publisher] Connection lost — reconnecting...")
		if err := p.connect(); err != nil {
			return err
		}
	}

	body, err := json.Marshal(listing)
	if err != nil {
		return fmt.Errorf("JSON marshal error: %w", err)
	}

	// Publish with persistent delivery mode (2) so messages survive restart
	return p.channel.Publish(
		"",    // Default exchange
		queue, // Routing key = queue name
		false, // Mandatory
		false, // Immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // Survive broker restarts
			Body:         body,
		},
	)
}

// Close gracefully closes the AMQP connection
func (p *Publisher) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil && !p.conn.IsClosed() {
		p.conn.Close()
	}
}
