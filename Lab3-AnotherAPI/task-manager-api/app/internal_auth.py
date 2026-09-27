import os

from dotenv import load_dotenv
from fastapi import Header, HTTPException


load_dotenv()

INTERNAL_API_KEY = os.getenv("INTERNAL_API_KEY")


def verify_internal_api_key(
    x_internal_key: str | None = Header(default=None)
):
    """
    Проверяет API-ключ для внутренних endpoint'ов.
    """

    if not INTERNAL_API_KEY:
        raise HTTPException(
            status_code=500,
            detail="Внутренний API-ключ не настроен"
        )

    if x_internal_key != INTERNAL_API_KEY:
        raise HTTPException(
            status_code=403,
            detail="Недействительный внутренний API-ключ"
        )

    return True