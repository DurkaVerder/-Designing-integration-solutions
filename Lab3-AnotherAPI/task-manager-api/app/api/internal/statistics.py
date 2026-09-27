from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from app.database import get_db
from app.internal_auth import verify_internal_api_key
from app.models import Task


router = APIRouter(
    prefix="/tasks",
    tags=["Internal API"]
)


@router.get(
    "/statistics",
    dependencies=[Depends(verify_internal_api_key)]
)
def get_task_statistics(
    db: Session = Depends(get_db)
):
    """
    Внутренняя статистика по задачам.

    Endpoint предназначен для внутренних сервисов,
    административной панели и мониторинга.
    """

    total = db.query(Task).count()

    todo = db.query(Task).filter(
        Task.status == "todo"
    ).count()

    in_progress = db.query(Task).filter(
        Task.status == "in_progress"
    ).count()

    done = db.query(Task).filter(
        Task.status == "done"
    ).count()

    overdue = 0

    tasks_with_deadline = db.query(Task).filter(
        Task.due_date.isnot(None),
        Task.status != "done"
    ).all()

    from datetime import datetime

    now = datetime.now()

    for task in tasks_with_deadline:
        if task.due_date < now:
            overdue += 1

    return {
        "total": total,
        "todo": todo,
        "in_progress": in_progress,
        "done": done,
        "overdue": overdue
    }