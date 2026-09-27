import time

from fastapi import Request
from fastapi.responses import JSONResponse


RATE_LIMIT = 100
WINDOW_SECONDS = 60


request_history: dict[str, list[float]] = {}


async def rate_limit_middleware(request: Request, call_next):
    """
    Ограничивает количество API-запросов
    с одного IP-адреса.

    Лимит:
    10 запросов за 60 секунд.
    """

    # Ограничиваем только API-эндпоинты
    if not request.url.path.startswith("/api/"):
        return await call_next(request)

    client_ip = request.client.host if request.client else "unknown"

    current_time = time.time()

    timestamps = request_history.get(client_ip, [])

    # Удаляем запросы, которые вышли за предел окна
    timestamps = [
        timestamp
        for timestamp in timestamps
        if current_time - timestamp < WINDOW_SECONDS
    ]

    # Проверяем превышение лимита
    if len(timestamps) >= RATE_LIMIT:
        oldest_request = timestamps[0]

        retry_after = int(
            WINDOW_SECONDS - (current_time - oldest_request)
        ) + 1

        response = JSONResponse(
            status_code=429,
            content={
                "detail": "Слишком много запросов. Попробуйте позже."
            }
        )

        response.headers["X-Limit-Remaining"] = "0"
        response.headers["Retry-After"] = str(retry_after)

        return response

    # Добавляем текущий запрос
    timestamps.append(current_time)
    request_history[client_ip] = timestamps

    # Передаём запрос дальше
    response = await call_next(request)

    # Сколько запросов осталось
    remaining = max(
        0,
        RATE_LIMIT - len(timestamps)
    )

    response.headers["X-Limit-Remaining"] = str(remaining)

    return response