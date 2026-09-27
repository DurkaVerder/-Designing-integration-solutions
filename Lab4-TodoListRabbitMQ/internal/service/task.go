package service

import (
	"context"
	"log"

	"TodoList/internal/entity"
)

type TaskEventPublisher interface {
	PublishTaskEvent(context.Context, string, entity.Task) error
}

type TaskRepository interface {
	GetTaskByID(id string) (entity.Task, error)
	GetTasks(userID string) ([]entity.Task, error)
	CreateTask(task entity.Task) (entity.Task, error)
	UpdateTask(task entity.Task) (entity.Task, error)
	DeleteTask(id string) error
}

type TaskService struct {
	repo      TaskRepository
	publisher TaskEventPublisher
}

func NewTaskService(repo TaskRepository, publishers ...TaskEventPublisher) *TaskService {
	service := &TaskService{repo: repo}
	if len(publishers) > 0 {
		service.publisher = publishers[0]
	}
	return service
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
	created, err := s.repo.CreateTask(task)
	if err == nil {
		s.publish("task.created", created)
	}
	return created, err
}

func (s *TaskService) UpdateTask(task entity.Task) (entity.Task, error) {
	updated, err := s.repo.UpdateTask(task)
	if err == nil {
		s.publish("task.updated", updated)
	}
	return updated, err
}

func (s *TaskService) DeleteTask(id string) error {
	err := s.repo.DeleteTask(id)
	if err == nil {
		s.publish("task.deleted", entity.Task{ID: id})
	}
	return err
}

func (s *TaskService) publish(eventType string, task entity.Task) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.PublishTaskEvent(context.Background(), eventType, task); err != nil {
		log.Printf("publish %s event: %v", eventType, err)
	}
}
