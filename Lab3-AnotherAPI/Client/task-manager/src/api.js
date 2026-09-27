const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8000';

// Общая обёртка для всех запросов: добавляет JWT и показывает ошибки API.
async function request(path, options = {}) {
  const token = localStorage.getItem('task-manager-token');
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    const rateLimit = response.status === 429 ? ` Повторите через ${response.headers.get('Retry-After') || '?'} сек.` : '';
    throw new Error(`${error.detail || `Ошибка ${response.status}`}${rateLimit}`);
  }
  return response.status === 204 ? null : response.json();
}

// Отправка JSON-тела для POST и PUT запросов.
const json = (method, path, data, headers = {}) => request(path, { method, body: JSON.stringify(data), headers });

export const api = {
  // Авторизация и регистрация пользователя.
  login: (data) => json('POST', '/api/v1/auth/login', data),
  register: (data) => json('POST', '/api/v1/auth/register', data),

  // CRUD пользователей.
  listUsers: () => request('/api/v1/users/'),
  getUser: (id) => request(`/api/v1/users/${id}`),
  createUser: (data) => json('POST', '/api/v1/users/', data),
  updateUser: (id, data) => json('PUT', `/api/v1/users/${id}`, data),
  deleteUser: (id) => request(`/api/v1/users/${id}`, { method: 'DELETE' }),

  // Проекты. Для POST нужен Idempotency-Key, чтобы избежать дублей.
  getProject: (id) => request(`/api/v1/projects/${id}`),
  createProject: (data, key) => json('POST', '/api/v1/projects/', data, { 'Idempotency-Key': key }),
  updateProject: (id, data) => json('PUT', `/api/v1/projects/${id}`, data),
  deleteProject: (id) => request(`/api/v1/projects/${id}`, { method: 'DELETE' }),

  // Базовые операции с задачами в API v1.
  listTasks: () => request('/api/v1/tasks/'),
  getTask: (id) => request(`/api/v1/tasks/${id}`),
  createTask: (data) => json('POST', '/api/v1/tasks/', data),
  updateTask: (id, data) => json('PUT', `/api/v1/tasks/${id}`, data),
  deleteTask: (id) => request(`/api/v1/tasks/${id}`, { method: 'DELETE' }),

  // API v2: фильтры, пагинация и дополнительные данные проекта/исполнителя.
  listTasksV2: (params) => request(`/api/v2/tasks/?${new URLSearchParams(
    Object.entries(params).filter(([, value]) => value !== '' && value !== null && value !== undefined)
  )}`),
  getTaskV2: (id) => request(`/api/v2/tasks/${id}`),
  createTaskV2: (data, key) => json('POST', '/api/v2/tasks/', data, key ? { 'Idempotency-Key': key } : {}),
  updateTaskV2: (id, data) => json('PUT', `/api/v2/tasks/${id}`, data),
  deleteTaskV2: (id) => request(`/api/v2/tasks/${id}`, { method: 'DELETE' }),

  // Внутренняя статистика требует отдельный X-Internal-Key.
  statistics: (key) => request('/api/internal/tasks/statistics', { headers: { 'X-Internal-Key': key } }),
};