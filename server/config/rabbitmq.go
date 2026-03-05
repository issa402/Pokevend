// ============================================================
// config/rabbitmq.go — RabbitMQ AMQP connection
// ============================================================
package config

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

func ConnectRabbitMQ(cfg *Config) (*amqp.Connection, error) {
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
