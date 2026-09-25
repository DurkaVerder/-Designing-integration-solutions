package service

import (
	"TodoList/internal/entity"
)

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

func (s *TaskService) GetTasks(userID string, offset int, limit int) ([]entity.Task, error) {
	tasks, err := s.repo.GetTasks(userID)
	if err != nil {
		return nil, err
	}
	if offset >= len(tasks) {
		return []entity.Task{}, nil
	}
	end := offset + limit
	if end > len(tasks) {
		end = len(tasks)
	}
	return tasks[offset:end], nil
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
