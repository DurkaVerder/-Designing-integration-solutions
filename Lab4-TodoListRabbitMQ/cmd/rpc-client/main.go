package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"time"

	rabbitmq "TodoList/internal/broker/rabbitMQ"
)

func main() {
	url := flag.String("url", "amqp://guest:guest@localhost:5672/", "RabbitMQ URL")
	version := flag.String("version", "v1", "API version")
	action := flag.String("action", "", "RPC action")
	data := flag.String("data", "{}", "JSON data")
	auth := flag.String("auth", "development-api-key", "API key")
	timeout := flag.Duration("timeout", 10*time.Second, "response timeout")
	flag.Parse()

	if *action == "" {
		log.Fatal("-action is required: create_user, get_user, update_user or delete_user")
	}

	client, err := rabbitmq.NewClient(rabbitmq.DefaultConfig(*url))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	requestID, err := newRequestID()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	response, err := client.Call(ctx, rabbitmq.RPCRequest{
		ID:      requestID,
		Version: *version,
		Action:  *action,
		Data:    []byte(*data),
		Auth:    *auth,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("correlation_id=%s status=%s\ndata=%v\nerror=%v\n", response.CorrelationID, response.Status, response.Data, response.Error)
}

func newRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(bytes[0:4]),
		hex.EncodeToString(bytes[4:6]),
		hex.EncodeToString(bytes[6:8]),
		hex.EncodeToString(bytes[8:10]),
		hex.EncodeToString(bytes[10:16]),
	), nil
}
