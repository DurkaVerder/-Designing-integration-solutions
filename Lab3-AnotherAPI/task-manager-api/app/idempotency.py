import json

from fastapi import HTTPException
from sqlalchemy.orm import Session

from app.models import IdempotencyRecord


def get_idempotency_record(
    db: Session,
    key: str,
    endpoint: str
):
    return db.query(IdempotencyRecord).filter(
        IdempotencyRecord.key == key,
        IdempotencyRecord.endpoint == endpoint
    ).first()


def save_idempotency_record(
    db: Session,
    key: str,
    endpoint: str,
    status_code: int,
    response_data: dict
):
    record = IdempotencyRecord(
        key=key,
        endpoint=endpoint,
        response_status=status_code,
        response_body=json.dumps(
            response_data,
            default=str,
            ensure_ascii=False
        )
    )

    db.add(record)
    db.commit()

    return record