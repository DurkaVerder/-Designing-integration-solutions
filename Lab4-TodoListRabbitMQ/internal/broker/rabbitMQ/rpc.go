package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"TodoList/internal/entity"

	"github.com/rabbitmq/amqp091-go"
)

const (
	RPCRequestsQueue   = "api.requests"
	RPCResponsesQueue  = "api.responses"
	RPCDeadLetterQueue = "api.requests.dlq"
	RPCMaxRetries      = 3
)

type RPCRequest struct {
	ID      string          `json:"id"`
	Version string          `json:"version"`
	Action  string          `json:"action"`
	Data    json.RawMessage `json:"data"`
	Auth    string          `json:"auth"`
}

type RPCResponse struct {
	CorrelationID string `json:"correlation_id"`
	Status        string `json:"status"`
	Data          any    `json:"data,omitempty"`
	Error         any    `json:"error"`
}

type RPCUserService interface {
	GetUserByID(string) (entity.User, error)
	CreateUser(entity.User) (entity.User, error)
	UpdateUser(entity.User) (entity.User, error)
	DeleteUser(string) error
}

type RPCTaskService interface {
	GetTaskByID(string) (entity.Task, error)
	GetTasks(string, int, int) ([]entity.Task, error)
	CreateTask(entity.Task) (entity.Task, error)
	UpdateTask(entity.Task) (entity.Task, error)
	DeleteTask(string) error
}

func (c *Client) declareRPCTopology() error {
	channel, err := c.connection.Channel()
	if err != nil {
		return fmt.Errorf("open RPC topology channel: %w", err)
	}
	defer channel.Close()

	if _, err := channel.QueueDeclare(RPCDeadLetterQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare RPC dead-letter queue: %w", err)
	}
	deadLetterArgs := amqp091.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": RPCDeadLetterQueue,
	}
	if _, err := channel.QueueDeclare(RPCRequestsQueue, true, false, false, false, deadLetterArgs); err != nil {
		return fmt.Errorf("declare RPC request queue: %w", err)
	}
	if _, err := channel.QueueDeclare(RPCResponsesQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare RPC response queue: %w", err)
	}
	return nil
}

func (c *Client) PublishRequest(ctx context.Context, request RPCRequest) (string, error) {
	if request.ID == "" {
		return "", errors.New("request id is required")
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal RPC request: %w", err)
	}
	channel, err := c.connection.Channel()
	if err != nil {
		return "", fmt.Errorf("open RPC publisher channel: %w", err)
	}
	defer channel.Close()

	if err := channel.PublishWithContext(ctx, "", RPCRequestsQueue, false, false, amqp091.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp091.Persistent,
		CorrelationId: request.ID,
		ReplyTo:       RPCResponsesQueue,
		Body:          payload,
	}); err != nil {
		return "", fmt.Errorf("publish RPC request: %w", err)
	}
	return request.ID, nil
}

func (c *Client) AwaitResponse(ctx context.Context, correlationID string) (RPCResponse, error) {
	channel, err := c.connection.Channel()
	if err != nil {
		return RPCResponse{}, fmt.Errorf("open RPC response channel: %w", err)
	}
	defer channel.Close()
	deliveries, err := channel.Consume(RPCResponsesQueue, "", false, false, false, false, nil)
	if err != nil {
		return RPCResponse{}, fmt.Errorf("consume RPC response: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return RPCResponse{}, ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return RPCResponse{}, errors.New("RPC response channel closed")
			}
			if delivery.CorrelationId != correlationID {
				if err := delivery.Nack(false, true); err != nil {
					return RPCResponse{}, fmt.Errorf("return unrelated RPC response: %w", err)
				}
				continue
			}
			var response RPCResponse
			if err := json.Unmarshal(delivery.Body, &response); err != nil {
				_ = delivery.Reject(false)
				return RPCResponse{}, fmt.Errorf("decode RPC response: %w", err)
			}
			if err := delivery.Ack(false); err != nil {
				return RPCResponse{}, fmt.Errorf("acknowledge RPC response: %w", err)
			}
			return response, nil
		}
	}
}

type RPCServer struct {
	client       *Client
	users        RPCUserService
	tasks        RPCTaskService
	expectedAuth string
	mu           sync.Mutex
	processed    map[string]RPCResponse
}

func NewRPCServer(client *Client, users RPCUserService, tasks RPCTaskService, expectedAuth string) *RPCServer {
	return &RPCServer{client: client, users: users, tasks: tasks, expectedAuth: expectedAuth, processed: make(map[string]RPCResponse)}
}

func (s *RPCServer) Serve(ctx context.Context) error {
	channel, err := s.client.connection.Channel()
	if err != nil {
		return fmt.Errorf("open RPC server channel: %w", err)
	}
	defer channel.Close()
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("configure RPC QoS: %w", err)
	}
	deliveries, err := channel.Consume(RPCRequestsQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start RPC server: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("RPC request channel closed")
			}
			if err := s.processDelivery(ctx, channel, delivery); err != nil {
				log.Printf("RPC message processing failed: %v", err)
			}
		}
	}
}

func (s *RPCServer) processDelivery(ctx context.Context, channel *amqp091.Channel, delivery amqp091.Delivery) error {
	var request RPCRequest
	if err := json.Unmarshal(delivery.Body, &request); err != nil {
		return s.deadLetter(channel, delivery, "malformed request")
	}
	if request.ID == "" {
		return s.deadLetter(channel, delivery, "missing request id")
	}
	if response, ok := s.cached(request.ID); ok {
		return s.respond(channel, delivery, response)
	}
	if request.Auth != s.expectedAuth {
		response := RPCResponse{CorrelationID: request.ID, Status: "error", Error: "unauthorized"}
		return s.deadLetterWithResponse(channel, delivery, response, "unauthorized")
	}

	response, permanent := s.handle(request)
	if response.Status == "ok" {
		return s.respondAndCache(channel, delivery, response)
	}
	if permanent {
		return s.deadLetterWithResponse(channel, delivery, response, "unprocessable request")
	}
	if retryCount(delivery.Headers) < RPCMaxRetries {
		return s.retry(channel, delivery)
	}
	return s.deadLetterWithResponse(channel, delivery, response, "retry limit exceeded")
}

func (s *RPCServer) handle(request RPCRequest) (RPCResponse, bool) {
	response := RPCResponse{CorrelationID: request.ID, Status: "error"}
	if request.Version != "v1" && request.Version != "v2" {
		response.Error = "unsupported version"
		return response, true
	}
	switch request.Action {
	case "create_user":
		var user entity.User
		if err := json.Unmarshal(request.Data, &user); err != nil {
			response.Error = "invalid user data"
			return response, true
		}
		created, err := s.users.CreateUser(user)
		if err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", created, nil
	case "get_user":
		var data struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(request.Data, &data); err != nil || data.ID == "" {
			response.Error = "user id is required"
			return response, true
		}
		user, err := s.users.GetUserByID(data.ID)
		if err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", user, nil
	case "update_user":
		var user entity.User
		if err := json.Unmarshal(request.Data, &user); err != nil || user.ID == "" {
			response.Error = "user id is required"
			return response, true
		}
		updated, err := s.users.UpdateUser(user)
		if err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", updated, nil
	case "delete_user":
		var data struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(request.Data, &data); err != nil || data.ID == "" {
			response.Error = "user id is required"
			return response, true
		}
		if err := s.users.DeleteUser(data.ID); err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", map[string]string{"id": data.ID}, nil
	case "create_task":
		var task entity.Task
		if err := json.Unmarshal(request.Data, &task); err != nil || task.CreatedUserID == "" {
			response.Error = "created_user_id is required"
			return response, true
		}
		created, err := s.tasks.CreateTask(task)
		if err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", created, nil
	case "get_task":
		var data struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(request.Data, &data); err != nil || data.ID == "" {
			response.Error = "task id is required"
			return response, true
		}
		task, err := s.tasks.GetTaskByID(data.ID)
		if err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", task, nil
	case "get_tasks":
		var data struct {
			UserID string `json:"user_id"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal(request.Data, &data); err != nil || data.UserID == "" {
			response.Error = "user_id is required"
			return response, true
		}
		if data.Limit <= 0 {
			data.Limit = 10
		}
		tasks, err := s.tasks.GetTasks(data.UserID, data.Offset, data.Limit)
		if err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", tasks, nil
	case "update_task":
		var task entity.Task
		if err := json.Unmarshal(request.Data, &task); err != nil || task.ID == "" || task.CreatedUserID == "" {
			response.Error = "task id and created_user_id are required"
			return response, true
		}
		updated, err := s.tasks.UpdateTask(task)
		if err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", updated, nil
	case "delete_task":
		var data struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(request.Data, &data); err != nil || data.ID == "" {
			response.Error = "task id is required"
			return response, true
		}
		if err := s.tasks.DeleteTask(data.ID); err != nil {
			response.Error = err.Error()
			return response, isPermanentError(err)
		}
		response.Status, response.Data, response.Error = "ok", map[string]string{"id": data.ID}, nil
	default:
		response.Error = "unsupported action"
		return response, true
	}
	return response, false
}

func (s *RPCServer) cached(id string) (RPCResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	response, ok := s.processed[id]
	return response, ok
}

func (s *RPCServer) respondAndCache(channel *amqp091.Channel, delivery amqp091.Delivery, response RPCResponse) error {
	if err := s.respond(channel, delivery, response); err != nil {
		return err
	}
	s.mu.Lock()
	s.processed[response.CorrelationID] = response
	s.mu.Unlock()
	return nil
}

func (s *RPCServer) respond(channel *amqp091.Channel, delivery amqp091.Delivery, response RPCResponse) error {
	if err := s.publishResponse(channel, delivery, response); err != nil {
		return err
	}
	return delivery.Ack(false)
}

func (s *RPCServer) publishResponse(channel *amqp091.Channel, delivery amqp091.Delivery, response RPCResponse) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal RPC response: %w", err)
	}
	if delivery.ReplyTo != "" {
		if err := channel.PublishWithContext(context.Background(), "", delivery.ReplyTo, false, false, amqp091.Publishing{
			ContentType: "application/json", CorrelationId: delivery.CorrelationId, Body: payload,
		}); err != nil {
			return fmt.Errorf("publish RPC response: %w", err)
		}
	}
	return nil
}

func (s *RPCServer) retry(channel *amqp091.Channel, delivery amqp091.Delivery) error {
	headers := delivery.Headers
	if headers == nil {
		headers = amqp091.Table{}
	}
	headers["x-retry-count"] = retryCount(headers) + 1
	if err := channel.PublishWithContext(context.Background(), "", RPCRequestsQueue, false, false, amqp091.Publishing{
		ContentType: "application/json", DeliveryMode: amqp091.Persistent, Headers: headers,
		CorrelationId: delivery.CorrelationId, ReplyTo: delivery.ReplyTo, Body: delivery.Body,
	}); err != nil {
		return fmt.Errorf("republish RPC retry: %w", err)
	}
	return delivery.Ack(false)
}

func (s *RPCServer) deadLetter(channel *amqp091.Channel, delivery amqp091.Delivery, reason string) error {
	log.Printf("RPC message moved to DLQ: %s", reason)
	if err := publishDeadLetter(channel, delivery, reason); err != nil {
		return err
	}
	return delivery.Ack(false)
}

func (s *RPCServer) deadLetterWithResponse(channel *amqp091.Channel, delivery amqp091.Delivery, response RPCResponse, reason string) error {
	log.Printf("RPC message moved to DLQ: %s", reason)
	if err := publishDeadLetter(channel, delivery, reason); err != nil {
		return err
	}
	if err := s.publishResponse(channel, delivery, response); err != nil {
		return err
	}
	return delivery.Ack(false)
}

func publishDeadLetter(channel *amqp091.Channel, delivery amqp091.Delivery, reason string) error {
	if err := channel.PublishWithContext(context.Background(), "", RPCDeadLetterQueue, false, false, amqp091.Publishing{
		ContentType: "application/json", Headers: amqp091.Table{"x-error-reason": reason}, Body: delivery.Body,
	}); err != nil {
		return fmt.Errorf("publish RPC dead letter: %w", err)
	}
	return nil
}

func isPermanentError(err error) bool {
	message := err.Error()
	return strings.Contains(message, "resource not found") || strings.Contains(message, "email already registered")
}

func retryCount(headers amqp091.Table) int {
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

func (c *Client) Call(ctx context.Context, request RPCRequest) (RPCResponse, error) {
	correlationID, err := c.PublishRequest(ctx, request)
	if err != nil {
		return RPCResponse{}, err
	}
	return c.AwaitResponse(ctx, correlationID)
}
