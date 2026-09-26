# TodoList API (Lab 2)

REST API для регистрации пользователей, JWT-аутентификации и управления личными задачами. В проекте доступны две версии публичного API: `v1` и `v2`. Данные хранятся в памяти процесса и предназначены для учебного/демонстрационного использования.

## Возможности

- регистрация пользователя и вход по email и паролю;
- JWT-аутентификация защищённых запросов;
- CRUD-операции с профилями пользователей;
- CRUD-операции с задачами текущего пользователя;
- версионирование API через `/api/v1` и `/api/v2`;
- пагинация и выбор полей в `v2`;
- защита POST-запросов от повторной обработки через `Idempotency-Key`;
- ограничение частоты запросов: 60 запросов в минуту с одного IP;
- внутренний endpoint для межсервисного получения пользователя;
- OpenAPI-описание и Swagger UI.

## Технологии

- Go `1.25.6`;
- Gin `1.12.0`;
- JWT;
- in-memory repository;
- Docker и Docker Compose;
- OpenAPI 3.0.3.

## Структура проекта

```text
cmd/app/main.go                 Точка входа и регистрация маршрутов
internal/entity/                Модели пользователя и задачи
internal/handlers/v1/           Обработчики API v1
internal/handlers/v2/           Обработчики API v2
internal/handlers/intern/       Внутренние обработчики
internal/middleware/            CORS, JWT, rate limit и idempotency
internal/repository/in-memory/  Хранилище в памяти
internal/service/               Бизнес-логика
pkg/jwt/                        Создание и проверка JWT
docs/openapi.yaml               OpenAPI 3.0.3
Dockerfile                      Сборка контейнера API
docker-compose.yml              API и Swagger UI
```

## Быстрый старт через Docker Compose

Из каталога `Lab2-TodoList` выполните:

```bash
docker compose up --build
```

После запуска:

- API: <http://localhost:8080>
- проверка состояния: <http://localhost:8080/health>
- Swagger UI: <http://localhost:8082>
- исходная спецификация: [`docs/openapi.yaml`](docs/openapi.yaml)

Остановить контейнеры:

```bash
docker compose down
```

Для запуска в фоне:

```bash
docker compose up --build -d
```

## Общие правила API

Все запросы и ответы с JSON используют заголовок:

```http
Content-Type: application/json
```

Для защищённых endpoint’ов передавайте JWT:

```http
Authorization: Bearer <JWT>
```

Идентификатор пользователя и задачи в текущей реализации генерируется как строковое числовое значение: `1`, `2`, `3` и т. д. Каждый пользователь видит и изменяет только свои задачи.

### Коды ответа

| Код | Значение |
|---:|---|
| `200` | Успешное чтение или обновление |
| `201` | Ресурс создан |
| `204` | Ресурс удалён, тело ответа отсутствует |
| `400` | Некорректный JSON или параметры запроса |
| `401` | Отсутствует или недействителен JWT/сервисный токен |
| `404` | Ресурс не найден или принадлежит другому пользователю |
| `409` | Email уже зарегистрирован |
| `429` | Превышен лимит запросов |
| `500` | Внутренняя ошибка сервиса |

Ошибки возвращаются в формате:

```json
{
  "error": "invalid request"
}
```

## Жизненный цикл запроса

### 1. Проверить сервис

```bash
curl http://localhost:8080/health
```

Ответ:

```json
{"status":"ok"}
```

### 2. Зарегистрировать пользователя

Регистрация не требует JWT. Все поля обязательны, пароль должен содержать не менее 6 символов.

```bash
curl -X POST http://localhost:8080/api/v2/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"ivan","email":"ivan@example.com","password":"secret123"}'
```

Ответ `201 Created`:

```json
{
  "id": "1",
  "username": "ivan",
  "email": "ivan@example.com"
}
```

Пароль никогда не возвращается в JSON-ответе.

### 3. Выполнить вход

```bash
curl -X POST http://localhost:8080/api/v2/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"ivan@example.com","password":"secret123"}'
```

Ответ `200 OK` содержит пользователя и JWT:

```json
{
  "user": {
    "id": "1",
    "username": "ivan",
    "email": "ivan@example.com"
  },
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

Сохраните значение `token` и передавайте его в заголовке `Authorization`.

## Endpoint’ы

Вместо `{version}` используйте `v1` или `v2`.

### Аутентификация

| Метод | Путь | JWT | Описание |
|---|---|:---:|---|
| `POST` | `/api/{version}/auth/register` | Нет | Создать пользователя |
| `POST` | `/api/{version}/auth/login` | Нет | Проверить credentials и получить JWT |

Тело регистрации:

```json
{
  "username": "ivan",
  "email": "ivan@example.com",
  "password": "secret123"
}
```

Тело входа:

```json
{
  "email": "ivan@example.com",
  "password": "secret123"
}
```

### Пользователи

| Метод | Путь | JWT | Описание |
|---|---|:---:|---|
| `GET` | `/api/{version}/users/{id}` | Да | Получить пользователя |
| `PUT` | `/api/{version}/users/{id}` | Да | Полностью обновить пользователя |
| `DELETE` | `/api/{version}/users/{id}` | Да | Удалить пользователя |

Тело `PUT` такое же, как при регистрации:

```json
{
  "username": "ivan-petrov",
  "email": "ivan.petrov@example.com",
  "password": "newsecret123"
}
```

### Задачи

| Метод | Путь | JWT | Описание |
|---|---|:---:|---|
| `GET` | `/api/{version}/tasks` | Да | Получить задачи текущего пользователя |
| `POST` | `/api/{version}/tasks` | Да | Создать задачу |
| `GET` | `/api/{version}/tasks/{id}` | Да | Получить свою задачу |
| `PUT` | `/api/{version}/tasks/{id}` | Да | Полностью обновить задачу |
| `DELETE` | `/api/{version}/tasks/{id}` | Да | Удалить свою задачу |

Базовое тело задачи:

```json
{
  "title": "Подготовить отчёт",
  "description": "Финальная проверка",
  "status": "new"
}
```

Поддерживаемые значения `status`: `new`, `in_progress`, `done`.

В `v2` дополнительно поддерживается поле `priority` со значениями `low`, `medium`, `high`:

```json
{
  "title": "Подготовить отчёт",
  "description": "Финальная проверка",
  "status": "in_progress",
  "priority": "high"
}
```

`created_user_id` устанавливается сервером из JWT и не должен передаваться клиентом.

Пример создания задачи:

```bash
curl -X POST http://localhost:8080/api/v2/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: create-report-001" \
  -d '{"title":"Подготовить отчёт","description":"Финальная проверка","status":"new","priority":"high"}'
```

### Различия v1 и v2

| Возможность | v1 | v2 |
|---|:---:|:---:|
| CRUD пользователей | Да | Да |
| CRUD задач | Да | Да |
| Лимит списка задач | Фиксированный `10` | `limit` от `1` до `100` |
| Смещение списка | Нет | `offset`, по умолчанию `0` |
| Выбор полей через `include` | Нет | Да |
| Поле `priority` в контракте | Нет | Да |

В `v1` запрос `GET /api/v1/tasks` возвращает первые 10 задач пользователя.

В `v2` можно использовать пагинацию:

```bash
curl "http://localhost:8080/api/v2/tasks?offset=10&limit=20" \
  -H "Authorization: Bearer $TOKEN"
```

Параметры `offset` и `limit` должны быть целыми числами. Некорректный `offset` или `limit` возвращает `400`.

Для уменьшения размера ответа используйте `include`. Поля `id`, `title` и `created_user_id` возвращаются всегда, дополнительные поля перечисляются через запятую:

```bash
curl "http://localhost:8080/api/v2/tasks?include=status,priority" \
  -H "Authorization: Bearer $TOKEN"
```

Допустимые значения `include`: `description`, `status`, `priority`. Неизвестное поле возвращает `400`.

Также `include` доступен для одной задачи:

```bash
curl "http://localhost:8080/api/v2/tasks/1?include=description,status" \
  -H "Authorization: Bearer $TOKEN"
```

## Idempotency-Key

Заголовок `Idempotency-Key` можно передавать при регистрации и создании задачи:

```http
Idempotency-Key: 7f4c2d8e-6d4c-4a35-9a6f-123456789abc
```

Если повторно отправить запрос с тем же методом, путём и ключом, API вернёт сохранённый успешный ответ вместо повторного создания ресурса. Неуспешные ответы не сохраняются.

Хранилище ключей находится в памяти процесса и не переживает перезапуск API.

## Внутренний endpoint

Endpoint предназначен для межсервисных запросов и использует отдельный заголовок:

```http
X-Service-Token: secret-service-token
```

Получить пользователя:

```bash
curl http://localhost:8080/internal/users/1 \
  -H "X-Service-Token: secret-service-token"
```

Ответ содержит также `created_at`, если это поле заполнено сервисом. Запрос без корректного токена получает `401 Unauthorized`.

> В текущей реализации значение сервисного токена задано в коде middleware (`secret-service-token`) и не настраивается через переменную окружения. Для production такое поведение необходимо заменить конфигурацией секретов.

## Middleware и ограничения

- **JWT** применяется ко всем маршрутам `/users` и `/tasks` после endpoint’ов регистрации и входа.
- **Rate limit** применяется ко всем HTTP-запросам: не более 60 запросов за минутное окно с одного IP. При превышении возвращается `429`, а время ожидания передаётся в `Retry-After`.
- **CORS** разрешает origins из `CORS_ALLOWED_ORIGINS` и методы `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`.
- **Данные** и счётчики идентификаторов находятся только в памяти. Перезапуск контейнера удаляет пользователей, задачи и сохранённые idempotency-ответы.

## OpenAPI и Swagger UI

Спецификация хранится в [`docs/openapi.yaml`](docs/openapi.yaml). При запуске через Compose Swagger UI автоматически использует этот файл и доступен по адресу <http://localhost:8082>.

Для просмотра только API без Swagger UI можно открыть <http://localhost:8080/health> или импортировать `docs/openapi.yaml` в любой совместимый инструмент.

