package inmemory

import (
	"errors"
	"strconv"
	"sync"

	"TodoList/internal/entity"
)

var ErrNotFound = errors.New("resource not found")

type InMemoryRepository struct {
	mu       sync.RWMutex
	nextUser int
	nextTask int
	users    map[string]entity.User
	tasks    map[string]entity.Task
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{nextUser: 1, nextTask: 1, users: make(map[string]entity.User), tasks: make(map[string]entity.Task)}
}

func (r *InMemoryRepository) GetUserByID(id string) (entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return entity.User{}, ErrNotFound
	}
	return user, nil
}
func (r *InMemoryRepository) GetUserByEmail(email string) (entity.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return entity.User{}, ErrNotFound
}
func (r *InMemoryRepository) CreateUser(user entity.User) (entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if existing.Email == user.Email {
			return entity.User{}, errors.New("email already registered")
		}
	}
	user.ID = strconv.Itoa(r.nextUser)
	r.nextUser++
	r.users[user.ID] = user
	return user, nil
}
func (r *InMemoryRepository) UpdateUser(user entity.User) (entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[user.ID]; !ok {
		return entity.User{}, ErrNotFound
	}
	r.users[user.ID] = user
	return user, nil
}
func (r *InMemoryRepository) DeleteUser(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[id]; !ok {
		return ErrNotFound
	}
	delete(r.users, id)
	return nil
}
func (r *InMemoryRepository) GetTaskByID(id string) (entity.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[id]
	if !ok {
		return entity.Task{}, ErrNotFound
	}
	return task, nil
}
func (r *InMemoryRepository) GetTasks(userID string) ([]entity.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]entity.Task, 0)
	for _, task := range r.tasks {
		if task.CreatedUserID == userID {
			result = append(result, task)
		}
	}
	return result, nil
}
func (r *InMemoryRepository) CreateTask(task entity.Task) (entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task.ID = strconv.Itoa(r.nextTask)
	r.nextTask++
	r.tasks[task.ID] = task
	return task, nil
}
func (r *InMemoryRepository) UpdateTask(task entity.Task) (entity.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[task.ID]; !ok {
		return entity.Task{}, ErrNotFound
	}
	r.tasks[task.ID] = task
	return task, nil
}
func (r *InMemoryRepository) DeleteTask(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}
