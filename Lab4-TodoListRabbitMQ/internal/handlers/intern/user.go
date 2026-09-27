package intern

import (
	"TodoList/internal/entity"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserService interface {
	GetUserByID(string) (entity.User, error)
}

type InternalHandler struct{ service UserService }

func NewInternalHandler(service UserService) *InternalHandler {
	return &InternalHandler{service: service}
}

func (h *InternalHandler) GetUser(c *gin.Context) {
	user, err := h.service.GetUserByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	userResponse := struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
	}{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}

	c.JSON(http.StatusOK, userResponse)
}
