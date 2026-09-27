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

## RabbitMQ: теория и реализация

RabbitMQ не отправляет сообщения напрямую в очередь. Producer публикует сообщение в
`exchange`, а exchange маршрутизирует его в одну или несколько очередей по
`binding key`. Consumer читает сообщения из очереди.

В этой лабораторной используется `topic exchange`:

```text
API (producer)
  |
  | routing key: task.created
  v
todo.events (topic exchange)
  |
  | binding key: task.#
  v
todo.events.consumer (durable queue) -> consumer
```

Основные понятия:

| Понятие | Назначение в проекте |
|---|---|
| Exchange | `todo.events`, принимает события и маршрутизирует их |
| Topic | Сопоставляет routing key с шаблонами `*` и `#` |
| Routing key | `task.created`, `task.updated` или `task.deleted` |
| Binding key | `task.#` получает все события, начинающиеся с `task.` |
| Queue | `todo.events.consumer`, хранит сообщения до обработки |
| ACK | Consumer подтверждает успешно обработанное сообщение |
| NACK + requeue | Временная ошибка: RabbitMQ возвращает сообщение в очередь |
| Reject без requeue | Некорректный JSON удаляется, чтобы не зациклить ошибку |
| QoS prefetch=1 | Одному consumer выдаётся только одно неподтверждённое сообщение |

### Контракт события

После создания и изменения задачи API публикует JSON:

```json
{
  "type": "task.created",
  "occurred_at": "2026-09-27T12:00:00Z",
  "task_id": "7",
  "task": {
  "id": "7",
  "title": "Подготовить отчёт",
  "status": "new",
  "created_user_id": "1"
  }
}
```

Для удаления поле `task` содержит только идентификатор. Формат описан в
`internal/broker/rabbitMQ/publisher.go`, а обработка и подтверждение сообщения
находятся в `internal/broker/rabbitMQ/consumer.go`.

### Запуск и проверка

```bash
docker compose up --build
```

RabbitMQ AMQP доступен на `localhost:5672`, а web-интерфейс управления на
`http://localhost:15672` (`guest` / `guest`). В логах API после создания задачи
появится:

```text
received RabbitMQ event: type=task.created task_id=7
```

Для запуска API без RabbitMQ переменную `RABBITMQ_URL` можно не задавать. Для
подключения к другому broker используйте, например:

```bash
RABBITMQ_URL=amqp://user:password@localhost:5672/ go run ./cmd/app
```

### Почему нужны durable и persistent

`durable=true` сохраняет exchange и очередь после перезапуска RabbitMQ.
`DeliveryMode=Persistent` помечает опубликованное сообщение как постоянное.
Оба параметра нужны вместе: persistent-сообщение не защищает от потери, если
очередь временная, а durable-очередь не делает transient-сообщение постоянным.
Надёжность также зависит от диска, publisher confirms и политики отказоустойчивости
кластера.

### Гарантии доставки и идемпотентность

Consumer использует manual ACK и тем самым реализует типичную модель
`at-least-once`: при падении процесса до ACK сообщение будет доставлено повторно.
Обработчик поэтому должен быть идемпотентным. Например, перед выполнением операции
можно сохранить `message_id` в таблице обработанных сообщений и пропустить уже
известный идентификатор.

Текущий пример упрощён: publisher логирует ошибку после сохранения задачи. Это
может привести к расхождению между БД и RabbitMQ. В production обычно применяют
паттерн **Transactional Outbox**:

1. В одной транзакции сохраняют задачу и запись события в таблицу `outbox`.
2. Отдельный worker читает необработанные записи и публикует их в RabbitMQ.
3. После подтверждения publisher confirm запись помечается обработанной.

Так HTTP-запрос не теряет событие, если RabbitMQ временно недоступен.

### Минимальный пример consumer

```go
client, err := rabbitmq.NewClient(
  rabbitmq.DefaultConfig("amqp://guest:guest@localhost:5672/"),
)
if err != nil {
  log.Fatal(err)
}
defer client.Close()

ctx := context.Background()
err = client.Consume(ctx, func(ctx context.Context, event rabbitmq.Event) error {
  log.Printf("event=%s task=%s", event.Type, event.TaskID)
  return nil // ACK; ошибка вернула бы сообщение через NACK + requeue
})
```

Для отдельного сервиса следует использовать собственное имя очереди. Тогда
RabbitMQ отправит копию события каждому сервису, а одинаковые consumers с одним
именем очереди будут распределять работу между собой.

### RPC-взаимодействие API

Помимо событий задач, приложение поддерживает запрос-ответ через очереди:

```text
RPC client -> api.requests -> RPC server
RPC client <- api.responses <- RPC server
                         \
                          -> api.requests.dlq
```

Запрос публикуется с `correlation_id=request.id` и `reply_to=api.responses`.
Сервер копирует этот идентификатор в ответ, поэтому клиент может сопоставить
результат с исходным запросом.

Запрос `create_user`:

```json
{
  "id": "7b8b1f37-6f23-4cb5-8a66-0bb9e4a12345",
  "version": "v1",
  "action": "create_user",
  "data": {
    "username": "ivan",
    "email": "ivan@example.com",
    "password": "secret123"
  },
  "auth": "development-api-key"
}
```

Успешный ответ:

```json
{
  "correlation_id": "7b8b1f37-6f23-4cb5-8a66-0bb9e4a12345",
  "status": "ok",
  "data": {"id": "1", "username": "ivan", "email": "ivan@example.com"},
  "error": null
}
```

Поддерживаются действия `login_user`, `create_user`, `get_user`, `update_user`, `delete_user`,
`create_task`, `get_task`, `get_tasks`, `update_task` и `delete_task`. Поле `auth`
проверяется сервером по переменной `RABBITMQ_API_KEY`. Для лабораторной выбран
API-ключ: он проще JWT для machine-to-machine вызовов, не требует состояния
сессии и подходит для доверенной внутренней очереди. В production ключ следует
хранить в Secret Manager, ротировать и передавать RabbitMQ через TLS.

Для `get_tasks` дополнительно используется JWT пользователя. Клиент передаёт
токен в поле `token`, а сервер извлекает `user_id` из claims, поэтому передавать
`user_id` в `data` не требуется. API-ключ отвечает за доступ RPC-клиента к
сервису, JWT отвечает за идентификацию пользователя.

Идентификатор запроса является ключом идемпотентности. Сервер сохраняет ответ в
in-memory map и при повторном `id` возвращает тот же ответ без повторного вызова
сервиса. Это защищает от повторной доставки, но данные исчезают после перезапуска.
Для production map нужно заменить таблицей Redis или БД с уникальным индексом по
`request_id`.

При временной ошибке сервер публикует запрос обратно в `api.requests` с заголовком
`x-retry-count`. После трёх попыток исходное сообщение попадает в durable очередь
`api.requests.dlq`, а клиент получает ответ со статусом `error`. Некорректный JSON,
неверный API-ключ и неизвестное действие сразу считаются необрабатываемыми и
попадают в DLQ. Все такие случаи записываются в журнал приложения.

### Пример RPC-клиента на Go

```go
request := rabbitmq.RPCRequest{
    ID: "7b8b1f37-6f23-4cb5-8a66-0bb9e4a12345",
    Version: "v1",
    Action: "create_user",
    Data: json.RawMessage(`{"username":"ivan","email":"ivan@example.com","password":"secret123"}`),
    Auth: "development-api-key",
}

response, err := client.Call(context.Background(), request)
if err != nil {
    log.Fatal(err)
}
log.Printf("status=%s error=%v data=%v", response.Status, response.Error, response.Data)
```

Для запуска в фоне:

```bash
docker compose up --build -d
```

### Запуск готового RPC-клиента

Сначала запустите RabbitMQ и API:

```bash
docker compose up --build -d
```

Клиент находится в `cmd/rpc-client`. Его можно запускать из каталога Lab4:

```bash
go run ./cmd/rpc-client \
  -action create_user \
  -data '{"username":"ivan","email":"ivan@example.com","password":"secret123"}'
```

Ожидаемый результат:

```text
correlation_id=... status=ok
data=map[email:ivan@example.com id:1 username:ivan]
error=<nil>
```

Получить пользователя:

```bash
go run ./cmd/rpc-client \
  -action get_user \
  -data '{"id":"1"}'
```

Изменить пользователя:

```bash
go run ./cmd/rpc-client \
  -action update_user \
  -data '{"id":"1","username":"petr","email":"petr@example.com","password":"secret123"}'
```

Удалить пользователя:

```bash
go run ./cmd/rpc-client \
  -action delete_user \
  -data '{"id":"1"}'
```

Полезные параметры:

```text
-url      адрес RabbitMQ, по умолчанию amqp://guest:guest@localhost:5672/
-auth     API-ключ, по умолчанию development-api-key
-version  версия API, по умолчанию v1
-timeout  время ожидания ответа, по умолчанию 10s
```

Например, запрос с другим ключом:

```bash
go run ./cmd/rpc-client \
  -auth "$RABBITMQ_API_KEY" \
  -action get_user \
  -data '{"id":"1"}'
```

При использовании PowerShell вместо обратного слеша для переноса строки можно
написать команду одной строкой или использовать обратный апостроф `` ` ``.

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

