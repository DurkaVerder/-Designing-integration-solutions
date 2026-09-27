package v1

import (
	"TodoList/internal/entity"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TaskService interface {
	GetTaskByID(string) (entity.Task, error)
	GetTasks(string, int, int) ([]entity.Task, error)
	CreateTask(entity.Task) (entity.Task, error)
	UpdateTask(entity.Task) (entity.Task, error)
	DeleteTask(string) error
}
type TaskHandler struct{ service TaskService }

func NewTaskHandler(service TaskService) *TaskHandler { return &TaskHandler{service: service} }
func userID(c *gin.Context) (string, bool) {
	id, ok := c.Get("user_id")
	value, valid := id.(string)
	return value, ok && valid && value != ""
}
func (h *TaskHandler) GetTask(c *gin.Context) {
	task, err := h.service.GetTaskByID(c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	if id, ok := userID(c); !ok || task.CreatedUserID != id {
		c.JSON(404, gin.H{"error": "task not found"})
		return
	}
	c.JSON(200, task)
}
func (h *TaskHandler) GetAllTasks(c *gin.Context) {
	id, ok := userID(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authentication required"})
		return
	}
	tasks, err := h.service.GetTasks(id, 0, 10)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, tasks)
}
func (h *TaskHandler) CreateTask(c *gin.Context) {
	id, ok := userID(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authentication required"})
		return
	}
	var task entity.Task
	if c.ShouldBindJSON(&task) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	task.CreatedUserID = id
	created, err := h.service.CreateTask(task)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id, ok := userID(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authentication required"})
		return
	}
	var task entity.Task
	if c.ShouldBindJSON(&task) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	task.ID = c.Param("id")
	task.CreatedUserID = id
	updated, err := h.service.UpdateTask(task)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, updated)
}
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id, ok := userID(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authentication required"})
		return
	}
	task, err := h.service.GetTaskByID(c.Param("id"))
	if err != nil || task.CreatedUserID != id {
		c.JSON(404, gin.H{"error": "task not found"})
		return
	}
	if err = h.service.DeleteTask(c.Param("id")); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
