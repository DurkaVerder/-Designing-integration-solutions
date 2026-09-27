package main

import (
	rabbitmq "TodoList/internal/broker/rabbitMQ"
	"TodoList/internal/handlers/intern"
	v1 "TodoList/internal/handlers/v1"
	v2 "TodoList/internal/handlers/v2"
	"TodoList/internal/middleware"
	inmemory "TodoList/internal/repository/in-memory"
	"TodoList/internal/service"
	"TodoList/pkg/jwt"
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "development-secret"
	}
	jwtManager, err := jwt.NewManager(secret, 24*time.Hour, "todo-list")
	if err != nil {
		log.Fatal(err)
	}
	repo := inmemory.NewInMemoryRepository()
	users := service.NewUserService(repo, *jwtManager)
	var broker *rabbitmq.Client
	var taskPublisher service.TaskEventPublisher
	if url := os.Getenv("RABBITMQ_URL"); url != "" {
		var rabbitErr error
		broker, rabbitErr = rabbitmq.NewClientWithRetry(context.Background(), rabbitmq.DefaultConfig(url), 60, time.Second)
		if rabbitErr != nil {
			log.Printf("RabbitMQ disabled: %v", rabbitErr)
		} else {
			taskPublisher = broker
			rpcAPIKey := os.Getenv("RABBITMQ_API_KEY")
			if rpcAPIKey == "" {
				rpcAPIKey = "development-api-key"
			}
			go func() {
				err := broker.Consume(context.Background(), func(_ context.Context, event rabbitmq.Event) error {
					log.Printf("received RabbitMQ event: type=%s task_id=%s", event.Type, event.TaskID)
					return nil
				})
				if err != nil {
					log.Printf("RabbitMQ consumer stopped: %v", err)
				}
			}()
			defer broker.Close()
		}
	}
	tasks := service.NewTaskService(repo, taskPublisher)
	if broker != nil {
		rpcAPIKey := os.Getenv("RABBITMQ_API_KEY")
		if rpcAPIKey == "" {
			rpcAPIKey = "development-api-key"
		}
		rpcServer := rabbitmq.NewRPCServer(broker, users, tasks, jwtManager, rpcAPIKey)
		go func() {
			if err := rpcServer.Serve(context.Background()); err != nil {
				log.Printf("RabbitMQ RPC server stopped: %v", err)
			}
		}()
	}
	userHandler := v1.NewUserHandler(users)
	taskHandler := v1.NewTaskHandler(tasks)
	userHandlerV2 := v2.NewUserHandler(users)
	taskHandlerV2 := v2.NewTaskHandler(tasks)
	internalHandler := intern.NewInternalHandler(users)
	auth := middleware.NewMiddleware(jwtManager)
	limiter := middleware.NewRateLimiter(60, time.Minute)
	idempotency := middleware.NewIdempotencyStore()
	r := gin.Default()
	r.Use(middleware.CORS())
	r.Use(limiter.Handler())
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	registerV1 := func(prefix string) {
		api := r.Group(prefix)
		api.POST("/auth/register", idempotency.Handler(), userHandler.RegisterUser)
		api.POST("/auth/login", userHandler.LoginUser)
		api.Use(auth.LoginMiddleware())
		api.GET("/users/:id", userHandler.GetUser)
		api.PUT("/users/:id", userHandler.UpdateUser)
		api.DELETE("/users/:id", userHandler.DeleteUser)
		api.GET("/tasks", taskHandler.GetAllTasks)
		api.POST("/tasks", idempotency.Handler(), taskHandler.CreateTask)
		api.GET("/tasks/:id", taskHandler.GetTask)
		api.PUT("/tasks/:id", taskHandler.UpdateTask)
		api.DELETE("/tasks/:id", taskHandler.DeleteTask)
	}
	registerV2 := func(prefix string) {
		api := r.Group(prefix)
		api.POST("/auth/register", idempotency.Handler(), userHandlerV2.RegisterUser)
		api.POST("/auth/login", userHandlerV2.LoginUser)
		api.Use(auth.LoginMiddleware())
		api.GET("/users/:id", userHandlerV2.GetUser)
		api.PUT("/users/:id", userHandlerV2.UpdateUser)
		api.DELETE("/users/:id", userHandlerV2.DeleteUser)
		api.GET("/tasks", taskHandlerV2.GetAllTasks)
		api.POST("/tasks", idempotency.Handler(), taskHandlerV2.CreateTask)
		api.GET("/tasks/:id", taskHandlerV2.GetTask)
		api.PUT("/tasks/:id", taskHandlerV2.UpdateTask)
		api.DELETE("/tasks/:id", taskHandlerV2.DeleteTask)
	}
	registerInternal := func(prefix string) {
		api := r.Group(prefix)
		api.Use(auth.ServiceTokenMiddleware())
		api.GET("/users/:id", internalHandler.GetUser)
	}
	registerInternal("/internal")
	registerV1("/api/v1")
	registerV2("/api/v2")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("TodoList API listening on :%s", port)
	log.Fatal(r.Run(":" + port))
}
