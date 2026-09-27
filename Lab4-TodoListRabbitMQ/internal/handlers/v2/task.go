package v2

import (
	"TodoList/internal/entity"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const baseLimit = 10
const maxLimit = 100
const defaultOffset = 0
const defaultLimit = 10
const offsetParam = "offset"
const limitParam = "limit"
const includeParam = "include"

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

	include := c.Query(includeParam)
	if include == "" {
		c.JSON(200, task)
		return
	}

	fields := strings.Split(include, ",")
	result := make(map[string]interface{})
	for _, field := range fields {
		// Always include ID, Title, and CreatedUserID in the response
		result["id"] = task.ID
		result["title"] = task.Title
		result["created_user_id"] = task.CreatedUserID

		switch field {
		case "description":
			result["description"] = task.Description
		case "status":
			result["status"] = task.Status
		case "priority":
			result["priority"] = task.Priority
		default:
			c.JSON(400, gin.H{"error": "invalid include field: " + field})
			return
		}
	}

	c.JSON(200, result)

}

func (h *TaskHandler) GetAllTasks(c *gin.Context) {
	id, ok := userID(c)
	if !ok {
		c.JSON(401, gin.H{"error": "authentication required"})
		return
	}

	offsetStr := c.Query(offsetParam)
	limitStr := c.Query(limitParam)

	offset := defaultOffset
	limit := defaultLimit

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		} else {
			c.JSON(400, gin.H{"error": "invalid offset"})
			return
		}
	}

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= maxLimit {
			limit = parsedLimit
		} else {
			c.JSON(400, gin.H{"error": "invalid limit"})
			return
		}
	}

	tasks, err := h.service.GetTasks(id, offset, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	include := c.Query(includeParam)
	if include == "" {
		c.JSON(200, tasks)
		return
	}

	fields := strings.Split(include, ",")

	result := make([]map[string]interface{}, len(tasks))
	for i, task := range tasks {
		taskMap := make(map[string]interface{})
		for _, field := range fields {
			// Always include ID, Title, and CreatedUserID in the response
			taskMap["id"] = task.ID
			taskMap["title"] = task.Title
			taskMap["created_user_id"] = task.CreatedUserID

			switch field {
			case "description":
				taskMap["description"] = task.Description
			case "status":
				taskMap["status"] = task.Status
			case "priority":
				taskMap["priority"] = task.Priority
			default:
				c.JSON(400, gin.H{"error": "invalid include field: " + field})
				return
			}
		}
		result[i] = taskMap
	}

	c.JSON(200, result)
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
