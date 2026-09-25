package main

import (
	"TodoList/internal/handlers/intern"
	v1 "TodoList/internal/handlers/v1"
	v2 "TodoList/internal/handlers/v2"
	"TodoList/internal/middleware"
	inmemory "TodoList/internal/repository/in-memory"
	"TodoList/internal/service"
	"TodoList/pkg/jwt"
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
	tasks := service.NewTaskService(repo)
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
