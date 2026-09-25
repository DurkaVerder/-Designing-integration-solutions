package service

import "TodoList/internal/entity"

type TaskRepository interface {
	GetTaskByID(id string) (entity.Task, error)
	GetTasks(userID string) ([]entity.Task, error)
	CreateTask(task entity.Task) (entity.Task, error)
	UpdateTask(task entity.Task) (entity.Task, error)
	DeleteTask(id string) error
}

type TaskService struct {
	repo TaskRepository
}

func NewTaskService(repo TaskRepository) *TaskService {
	return &TaskService{repo: repo}
}

func (s *TaskService) GetTaskByID(id string) (entity.Task, error) {
	return s.repo.GetTaskByID(id)
}

func (s *TaskService) GetTasks(userID string) ([]entity.Task, error) {
	return s.repo.GetTasks(userID)
}

func (s *TaskService) CreateTask(task entity.Task) (entity.Task, error) {
	return s.repo.CreateTask(task)
}

func (s *TaskService) UpdateTask(task entity.Task) (entity.Task, error) {
	return s.repo.UpdateTask(task)
}

func (s *TaskService) DeleteTask(id string) error {
	return s.repo.DeleteTask(id)
}
