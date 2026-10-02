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
	ExchangeName    = "todo.events"
	ExchangeType    = "topic"
	QueueName       = "todo.events.consumer"
	RetryQueueName  = "todo.events.consumer.retry"
	DeadLetterQueue = "todo.events.consumer.dlq"
	BindingKey      = "task.#"
	RetryRoutingKey = "task.retry"
	MaxEventRetries = 3
	EventRetryDelay = 2 * time.Second
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

func NewClientWithRetry(ctx context.Context, config Config, attempts int, delay time.Duration) (*Client, error) {
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		client, err := NewClient(config)
		if err == nil {
			return client, nil
		}
		lastErr = err
		if attempt == attempts-1 {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("connect to RabbitMQ after %d attempts: %w", attempts, lastErr)
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
	deadLetterArgs := amqp091.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": DeadLetterQueue,
	}
	if _, err := channel.QueueDeclare(DeadLetterQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare event dead-letter queue: %w", err)
	}
	if _, err := channel.QueueDeclare(c.config.QueueName, true, false, false, false, deadLetterArgs); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	retryArgs := amqp091.Table{
		"x-message-ttl":             int(EventRetryDelay / time.Millisecond),
		"x-dead-letter-exchange":    c.config.ExchangeName,
		"x-dead-letter-routing-key": RetryRoutingKey,
	}
	if _, err := channel.QueueDeclare(RetryQueueName, true, false, false, false, retryArgs); err != nil {
		return fmt.Errorf("declare event retry queue: %w", err)
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
				log.Printf("ERROR event stage=decode queue=%s message_id=%s retry_count=%d body_bytes=%d reason=malformed_message error=%v", c.config.QueueName, delivery.MessageId, eventRetryCount(delivery.Headers), len(delivery.Body), err)
				if err := delivery.Reject(false); err != nil {
					return fmt.Errorf("reject malformed message: %w", err)
				}
				continue
			}
			if err := handler(ctx, event); err != nil {
				retries := eventRetryCount(delivery.Headers)
				if retries < MaxEventRetries {
					log.Printf("ERROR event stage=retry queue=%s message_id=%s type=%s retry_count=%d task_id=%s error=%v", c.config.QueueName, delivery.MessageId, event.Type, retries+1, event.TaskID, err)
					if err := publishEventRetry(channel, delivery, retries+1); err != nil {
						if nackErr := delivery.Nack(false, true); nackErr != nil {
							return fmt.Errorf("publish event retry: %v; requeue message: %w", err, nackErr)
						}
						return fmt.Errorf("publish event retry: %w", err)
					}
					if err := delivery.Ack(false); err != nil {
						return fmt.Errorf("acknowledge retried message: %w", err)
					}
					continue
				}
				log.Printf("ERROR event stage=dead_letter queue=%s message_id=%s type=%s retry_count=%d task_id=%s error=%v", DeadLetterQueue, delivery.MessageId, event.Type, retries, event.TaskID, err)
				if err := delivery.Reject(false); err != nil {
					return fmt.Errorf("move event to dead-letter queue: %w", err)
				}
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("acknowledge message: %w", err)
			}
		}
	}
}

func publishEventRetry(channel *amqp091.Channel, delivery amqp091.Delivery, retry int) error {
	headers := amqp091.Table{}
	for key, value := range delivery.Headers {
		headers[key] = value
	}
	headers["x-retry-count"] = retry
	if err := channel.PublishWithContext(context.Background(), "", RetryQueueName, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Headers:      headers,
		Body:         delivery.Body,
	}); err != nil {
		return fmt.Errorf("publish event retry: %w", err)
	}
	return nil
}

func eventRetryCount(headers amqp091.Table) int {
	switch value := headers["x-retry-count"].(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	}
	return 0
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
