package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"TodoList/internal/entity"

	"github.com/rabbitmq/amqp091-go"
)

func (c *Client) PublishTaskEvent(ctx context.Context, eventType string, task entity.Task) error {
	channel, err := c.connection.Channel()
	if err != nil {
		return fmt.Errorf("open publisher channel: %w", err)
	}
	defer channel.Close()

	payload, err := json.Marshal(Event{
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		Task:       task,
		TaskID:     task.ID,
	})
	if err != nil {
		return fmt.Errorf("marshal task event: %w", err)
	}

	return channel.PublishWithContext(ctx, c.config.ExchangeName, eventType, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         payload,
	})
}
