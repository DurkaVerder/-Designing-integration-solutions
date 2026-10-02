# Команды RPC-клиента



## Создание пользователя

```bash
go run ./cmd/rpc-client -action create_user -data '{"username":"ivan","email":"ivan@example.com","password":"secret123"}'
```

Ожидаемый результат:

```text
correlation_id=... status=ok
data=map[email:ivan@example.com id:1 username:ivan]
error=<nil>
```

## Авторизация пользователя через RabbitMQ

Команда возвращает JWT в поле `token`:

```bash
go run ./cmd/rpc-client \
	-action login_user \
	-data '{"email":"alex@example.com","password":"secret123"}'
```

Скопируйте полученный JWT и используйте его для получения задач:

```bash
go run ./cmd/rpc-client \
	-token "<JWT_TOKEN>" \
	-action get_tasks \
	-data '{"offset":0,"limit":10}'
```

## Получение пользователя

```bash
go run ./cmd/rpc-client -action get_user -data '{"id":"1"}'
```

## Изменение пользователя

```bash
go run ./cmd/rpc-client -action update_user -data '{"id":"1","username":"petr","email":"petr@example.com","password":"secret123"}'
```

## Удаление пользователя

```bash
go run ./cmd/rpc-client -action delete_user -data '{"id":"1"}'
```


## Работа с задачами


### Создание задачи

```bash
go run ./cmd/rpc-client -action create_task -data '{"title":"Подготовить отчёт","description":"Финальная проверка","status":"new","priority":"high","created_user_id":"1"}'
```

### Получение одной задачи

```bash
go run ./cmd/rpc-client -action get_task -data '{"id":"1"}'
```

### Получение списка задач пользователя

```bash
go run ./cmd/rpc-client -token "<JWT_TOKEN>" -action get_tasks -data '{"offset":0,"limit":10}'
```

### Изменение задачи

```bash
go run ./cmd/rpc-client -action update_task -data '{"id":"1","title":"Подготовить финальный отчёт","description":"Проверить приложение","status":"in_progress","priority":"medium","created_user_id":"1"}'
```

### Удаление задачи

```bash
go run ./cmd/rpc-client -action delete_task -data '{"id":"1"}'
```


## Проверка идемпотентности


```bash
go run ./cmd/rpc-client \
	-id "11111111-1111-4111-8111-111111111111" \
	-action create_user \
	-data '{"username":"idempotent-user","email":"idempotent@example.com","password":"secret123"}'
```

Повторить абсолютно ту же команду:

```bash
go run ./cmd/rpc-client \
	-id "11111111-1111-4111-8111-111111111111" \
	-action create_user \
	-data '{"username":"idempotent-user","email":"idempotent@example.com","password":"secret123"}'
```

Обе команды должны вернуть один и тот же `correlation_id` и один и тот же `id`
пользователя. Вторая команда не создаёт нового пользователя.

## Проверка Dead Letter Queue

### RPC-DLQ

Запрос с неподдерживаемым действием считается необрабатываемым и попадёт в
очередь `api.requests.dlq`:

```bash
go run ./cmd/rpc-client \
	-action unknown_action \
	-data '{}'
```

Ожидаемый ответ клиента:

```text
status=error
error=unsupported action
```

Другие примеры необрабатываемых RPC-запросов:

```bash
go run ./cmd/rpc-client -auth invalid-api-key -action get_user -data '{"id":"1"}'
go run ./cmd/rpc-client -action create_task -data '{"title":"Без владельца"}'
```
