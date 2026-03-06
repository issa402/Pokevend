// ============================================================
// FILE: server/config/rabbitmq.go
// TYPE: Infrastructure — RabbitMQ Connection
//
// WHAT IS RABBITMQ?
// RabbitMQ is a message broker — a middleman between services.
// Instead of Service A calling Service B directly via HTTP,
// A drops a message into RabbitMQ; B picks it up when ready.
//
// CORE CONCEPTS:
//   Producer  → publishes messages to RabbitMQ exchange
//   Consumer  → subscribes to a queue and receives messages
//   Queue     → durable FIFO buffer for messages
//   Exchange  → routes messages from producers to queues
//   Binding   → connects an exchange to a queue
//
// OUR FLOW:
//   Python (api-consumer)       → publishes to "listings" queue
//   Go (notification_worker.go) → consumes from "listings" queue
//
// WHY MESSAGE QUEUE INSTEAD OF DIRECT HTTP CALL?
//   Direct HTTP: if Go API is down, Python's publish fails immediately
//   Message queue: if Go is down, messages QUEUE UP. When Go restarts,
//                  it processes all queued messages — zero data loss.
//
// AMQP 0-9-1 PROTOCOL:
// RabbitMQ uses AMQP (Advanced Message Queuing Protocol).
// "amqp091-go" is Go's client library for this protocol.
// Python's "aio-pika" uses the same protocol — fully compatible.
// They're speaking the same language (AMQP) to the same broker.
//
// amqp.Dial returns a *amqp.Connection which is long-lived.
// Create channels from it as needed (see worker/notification_worker.go).
// ============================================================
package config

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// ConnectRabbitMQ creates a single connection to the RabbitMQ broker.
// Called once in main.go; the connection is shared with the notification worker.
//
// Returns (*amqp.Connection, error):
//   - error if RabbitMQ is unreachable (handled gracefully in main.go)
//   - nil error + valid connection = ready to create channels and publish/consume
//
// Connection vs Channel:
//   Connection = the TCP connection to RabbitMQ (expensive to create)
//   Channel    = lightweight logical sub-connection (create many per connection)
//   Pattern: one Connection per service, one Channel per goroutine
func ConnectRabbitMQ(cfg *Config) (*amqp.Connection, error) {
	// amqp.Dial establishes the TCP connection to RabbitMQ.
	// URL format: amqp://user:password@host:port/vhost
	// Default dev creds: amqp://guest:guest@rabbitmq:5672
	// In production: use strong credentials via RABBITMQ_URL env var
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// TODO #1 (Practice): Add connection health monitoring
// AMQP connections can drop silently (network blip, broker restart).
// The amqp.Connection has a NotifyClose() method that returns a channel.
// Subscribe to it and reconnect automatically:
//   closeChan := conn.NotifyClose(make(chan *amqp.Error, 1))
//   go func() {
//     for range closeChan { reconnect() }
//   }()
// Research: "RabbitMQ Go reconnect pattern" — this is production-critical.

// TODO #2 (Practice): Create a shared RabbitMQ connection manager struct
// Instead of passing *amqp.Connection directly, create a wrapper:
//   type RabbitMQClient struct { conn *amqp.Connection }
//   func (r *RabbitMQClient) Channel() (*amqp.Channel, error)
//   func (r *RabbitMQClient) Publish(queue, body string) error
// This hides AMQP details from the worker and makes it testable (interface).
// Similar to how we wrapped PostgreSQL in pgxpool.Pool.
