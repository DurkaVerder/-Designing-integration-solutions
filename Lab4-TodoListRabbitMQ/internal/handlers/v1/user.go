package v1

import (
	"TodoList/internal/entity"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserService interface {
	GetUserByID(string) (entity.User, error)
	CreateUser(entity.User) (entity.User, error)
	UpdateUser(entity.User) (entity.User, error)
	LoginUser(string, string) (entity.User, string, error)
	DeleteUser(string) error
}
type UserHandler struct{ service UserService }

func NewUserHandler(service UserService) *UserHandler { return &UserHandler{service: service} }

type credentials struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}
type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *UserHandler) GetUser(c *gin.Context) {
	user, err := h.service.GetUserByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}
func (h *UserHandler) RegisterUser(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	user, err := h.service.CreateUser(entity.User{Username: req.Username, Email: req.Email, Password: req.Password})
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, user)
}
func (h *UserHandler) LoginUser(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	user, token, err := h.service.LoginUser(req.Email, req.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	c.JSON(200, gin.H{"user": user, "token": token})
}
func (h *UserHandler) UpdateUser(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	user, err := h.service.UpdateUser(entity.User{ID: c.Param("id"), Username: req.Username, Email: req.Email, Password: req.Password})
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, user)
}
func (h *UserHandler) DeleteUser(c *gin.Context) {
	if err := h.service.DeleteUser(c.Param("id")); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
