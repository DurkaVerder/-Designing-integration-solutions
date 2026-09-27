# Команды RPC-клиента

Все команды выполняются из каталога `Lab4-TodoListRabbitMQ`.

## 1. Запуск RabbitMQ и API

```bash
docker compose up --build -d
```

Проверка состояния API:

```bash
curl http://localhost:8080/health
```

Ожидаемый ответ:

```json
{"status":"ok"}
```

## 2. Справка клиента

```bash
go run ./cmd/rpc-client -h
```

## 3. Создание пользователя

```bash
go run ./cmd/rpc-client -action create_user -data '{"username":"ivan","email":"ivan@example.com","password":"secret123"}'
```

Ожидаемый результат:

```text
correlation_id=... status=ok
data=map[email:ivan@example.com id:1 username:ivan]
error=<nil>
```

## 4. Получение пользователя

```bash
go run ./cmd/rpc-client -action get_user -data '{"id":"1"}'
```

## 5. Изменение пользователя

```bash
go run ./cmd/rpc-client -action update_user -data '{"id":"1","username":"petr","email":"petr@example.com","password":"secret123"}'
```

## 6. Удаление пользователя

```bash
go run ./cmd/rpc-client -action delete_user -data '{"id":"1"}'
```

## 7. Параметры клиента

```text
-url      адрес RabbitMQ, по умолчанию amqp://guest:guest@localhost:5672/
-auth     API-ключ, по умолчанию development-api-key
-version  версия API, по умолчанию v1
-timeout  время ожидания ответа, по умолчанию 10s
```

## 8. Работа с задачами

В примерах ниже пользователь с идентификатором `1` должен существовать.

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
go run ./cmd/rpc-client -action get_tasks -data '{"user_id":"1","offset":0,"limit":10}'
```

Поля `offset` и `limit` необязательны. Если `limit` не указан, используется
значение `10`.

### Изменение задачи

```bash
go run ./cmd/rpc-client -action update_task -data '{"id":"1","title":"Подготовить финальный отчёт","description":"Проверить приложение","status":"in_progress","priority":"medium","created_user_id":"1"}'
```

### Удаление задачи

```bash
go run ./cmd/rpc-client -action delete_task -data '{"id":"1"}'
```

Поддерживаемые RPC-действия:

```text
create_user   создать пользователя
get_user      получить пользователя
update_user   изменить пользователя
delete_user   удалить пользователя
create_task   создать задачу
get_task      получить задачу
get_tasks     получить список задач пользователя
update_task   изменить задачу
delete_task   удалить задачу
```

Пример с явным API-ключом:

```bash
go run ./cmd/rpc-client -auth "development-api-key" -action get_user -data '{"id":"1"}'
```

Пример с другим адресом RabbitMQ:

```bash
go run ./cmd/rpc-client -url "amqp://guest:guest@localhost:5672/" -action get_user -data '{"id":"1"}'
```

## 9. Остановка контейнеров

```bash
docker compose down
```

## PowerShell

В PowerShell команды можно запускать одной строкой. Одинарные кавычки вокруг JSON поддерживаются напрямую:

```powershell
go run ./cmd/rpc-client -action create_user -data '{"username":"ivan","email":"ivan@example.com","password":"secret123"}'
```
