package service

import (
	"TodoList/internal/entity"
	"TodoList/pkg/jwt"
	"fmt"
)

type UserRepository interface {
	GetUserByID(id string) (entity.User, error)
	GetUserByEmail(email string) (entity.User, error)
	CreateUser(user entity.User) (entity.User, error)
	UpdateUser(user entity.User) (entity.User, error)
	DeleteUser(id string) error
}

type UserService struct {
	repo       UserRepository
	JWTManager jwt.Manager
}

func NewUserService(repo UserRepository, jwtManager jwt.Manager) *UserService {
	return &UserService{repo: repo, JWTManager: jwtManager}
}

func (s *UserService) GetUserByID(id string) (entity.User, error) {
	return s.repo.GetUserByID(id)
}

func (s *UserService) CreateUser(user entity.User) (entity.User, error) {
	return s.repo.CreateUser(user)
}

func (s *UserService) UpdateUser(user entity.User) (entity.User, error) {
	return s.repo.UpdateUser(user)
}

func (s *UserService) DeleteUser(id string) error {
	return s.repo.DeleteUser(id)
}

func (s *UserService) LoginUser(email, password string) (entity.User, string, error) {
	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return entity.User{}, "", err
	}
	if user.Password != password {
		return entity.User{}, "", fmt.Errorf("invalid credentials")
	}

	token, err := s.JWTManager.GenerateToken(user.ID, map[string]interface{}{
		"username": user.Username,
		"email":    user.Email,
		"user_id":  user.ID,
	})
	if err != nil {
		return entity.User{}, "", err
	}

	return user, token, nil
}
