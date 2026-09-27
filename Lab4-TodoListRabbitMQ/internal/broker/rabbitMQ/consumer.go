package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName = "todo.events"
	ExchangeType = "topic"
	QueueName    = "todo.events.consumer"
	BindingKey   = "task.#"
)

type Config struct {
	URL          string
	ExchangeName string
	QueueName    string
	BindingKey   string
}

func DefaultConfig(url string) Config {
	return Config{
		URL:          url,
		ExchangeName: ExchangeName,
		QueueName:    QueueName,
		BindingKey:   BindingKey,
	}
}

type Client struct {
	connection *amqp091.Connection
	config     Config
}

func NewClient(config Config) (*Client, error) {
	connection, err := amqp091.Dial(config.URL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	client := &Client{connection: connection, config: config}
	if err := client.declareTopology(); err != nil {
		_ = connection.Close()
		return nil, err
	}
	if err := client.declareRPCTopology(); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return client, nil
}

func (c *Client) declareTopology() error {
	channel, err := c.connection.Channel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare(c.config.ExchangeName, ExchangeType, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if _, err := channel.QueueDeclare(c.config.QueueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	if err := channel.QueueBind(c.config.QueueName, c.config.BindingKey, c.config.ExchangeName, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}
	return nil
}

func (c *Client) Consume(ctx context.Context, handler func(context.Context, Event) error) error {
	channel, err := c.connection.Channel()
	if err != nil {
		return fmt.Errorf("open consumer channel: %w", err)
	}
	defer channel.Close()

	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("configure consumer QoS: %w", err)
	}
	deliveries, err := channel.Consume(c.config.QueueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("RabbitMQ delivery channel closed")
			}

			var event Event
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				log.Printf("reject malformed RabbitMQ message: %v", err)
				if err := delivery.Reject(false); err != nil {
					return fmt.Errorf("reject malformed message: %w", err)
				}
				continue
			}
			if err := handler(ctx, event); err != nil {
				log.Printf("RabbitMQ handler failed for %s: %v", event.Type, err)
				if err := delivery.Nack(false, true); err != nil {
					return fmt.Errorf("requeue message: %w", err)
				}
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("acknowledge message: %w", err)
			}
		}
	}
}

func (c *Client) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	return c.connection.Close()
}

type Event struct {
	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	Task       any       `json:"task,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
}
