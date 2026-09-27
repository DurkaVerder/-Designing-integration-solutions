from datetime import datetime
from math import ceil
import json
from fastapi import APIRouter, Depends, HTTPException, Query, Header
from sqlalchemy.orm import Session

from app.database import get_db
from app.dependencies import get_current_user
from app.models import Task, Project, User
from app.schemas import (
    TaskCreate,
    TaskV2Response,
    TaskV2ExtendedResponse,
    TaskPaginationResponse,
)
from app.idempotency import (
    get_idempotency_record,
    save_idempotency_record,
)

router = APIRouter(
    prefix="/tasks",
    tags=["Tasks V2"]
)


def calculate_task_deadline(task: Task) -> tuple[bool, int | None]:
    """
    Вычисляет информацию о сроке выполнения задачи.

    Возвращает:
    - is_overdue — просрочена ли задача;
    - days_until_due — количество дней до дедлайна.

    Если задача выполнена, она не считается просроченной.
    """

    if task.due_date is None:
        return False, None

    now = datetime.now()

    if task.status == "done":
        return False, 0

    difference = task.due_date - now

    days_until_due = difference.days
    is_overdue = difference.total_seconds() < 0

    return is_overdue, days_until_due


def task_to_v2_response(task: Task) -> dict:
    """
    Формирует расширенный ответ API V2.
    """

    is_overdue, days_until_due = calculate_task_deadline(task)

    return {
        "id": task.id,
        "project_id": task.project_id,
        "title": task.title,
        "description": task.description,
        "status": task.status,
        "priority": task.priority,
        "assignee_id": task.assignee_id,
        "created_at": task.created_at,
        "due_date": task.due_date,
        "is_overdue": is_overdue,
        "days_until_due": days_until_due,
    }


def task_to_extended_response(
    task: Task,
    include: set[str],
) -> dict:
    """
    Формирует расширенный ответ с дополнительными
    связанными объектами.
    """

    result = task_to_v2_response(task)

    if "project" in include:
        result["project"] = {
            "id": task.project.id,
            "name": task.project.name,
        }

    if "assignee" in include:
        if task.assignee:
            result["assignee"] = {
                "id": task.assignee.id,
                "username": task.assignee.username,
            }
        else:
            result["assignee"] = None

    return result


# CREATE TASK

@router.post(
    "/",
    response_model=TaskV2Response,
    status_code=201,
)
def create_task(
    task_data: TaskCreate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    idempotency_key: str | None = Header(
        default=None,
        alias="Idempotency-Key",
        description=(
            "Уникальный ключ для защиты от повторного "
            "создания задачи"
        ),
    ),
):
    """
    Создание задачи через API V2.

    Поддерживает идемпотентность через заголовок
    Idempotency-Key.

    Если запрос с таким ключом уже выполнялся,
    возвращается сохранённый результат вместо
    повторного создания задачи.
    """

    endpoint = "/api/v2/tasks/"

    # Проверяем Idempotency-Key

    if idempotency_key:

        existing_record = get_idempotency_record(
            db=db,
            key=idempotency_key,
            endpoint=endpoint,
        )

        if existing_record:
            return json.loads(existing_record.response_body)

    # Проверяем проект

    project = db.query(Project).filter(
        Project.id == task_data.project_id,
        Project.owner_id == current_user.id,
    ).first()

    if not project:
        raise HTTPException(
            status_code=404,
            detail="Проект не найден или доступ запрещён",
        )

    # Проверяем исполнителя

    if task_data.assignee_id is not None:

        assignee = db.query(User).filter(
            User.id == task_data.assignee_id
        ).first()

        if not assignee:
            raise HTTPException(
                status_code=404,
                detail="Пользователь-исполнитель не найден",
            )

    # Создаём задачу

    task = Task(
        project_id=task_data.project_id,
        title=task_data.title,
        description=task_data.description,
        status=task_data.status,
        priority=task_data.priority,
        assignee_id=task_data.assignee_id,
        due_date=task_data.due_date,
    )

    db.add(task)
    db.commit()
    db.refresh(task)

    response_data = task_to_v2_response(task)

    # Сохраняем результат для повторных запросов

    if idempotency_key:

        save_idempotency_record(
            db=db,
            key=idempotency_key,
            endpoint=endpoint,
            status_code=201,
            response_data=response_data,
        )

    return response_data


# GET TASKS

@router.get(
    "/",
    response_model=TaskPaginationResponse,
)
def get_tasks(
    page: int = Query(
        default=1,
        ge=1,
        description="Номер страницы, начиная с 1",
    ),
    limit: int = Query(
        default=10,
        ge=1,
        le=100,
        description="Количество задач на странице",
    ),
    status: str | None = Query(
        default=None,
        description=(
            "Фильтр задач по статусу: "
            "todo, in_progress, done"
        ),
    ),
    priority: str | None = Query(
        default=None,
        description=(
            "Фильтр задач по приоритету: "
            "low, medium, high"
        ),
    ),
    include: str | None = Query(
        default=None,
        description=(
            "Дополнительные данные: "
            "project, assignee. "
            "Можно указать несколько через запятую."
        ),
    ),
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """
    Получение списка задач с пагинацией,
    фильтрацией и опциональным включением
    связанных объектов.
    """

    include_set: set[str] = set()

    # INCLUDE

    if include:

        include_set = {
            item.strip().lower()
            for item in include.split(",")
            if item.strip()
        }

        allowed_includes = {
            "project",
            "assignee",
        }

        invalid_includes = include_set - allowed_includes

        if invalid_includes:
            raise HTTPException(
                status_code=400,
                detail=(
                    "Недопустимое значение include. "
                    "Используйте: project, assignee"
                ),
            )

    # BASE QUERY

    query = (
        db.query(Task)
        .join(Project)
        .filter(
            Project.owner_id == current_user.id
        )
    )

    # STATUS FILTER

    if status is not None:

        allowed_statuses = {
            "todo",
            "in_progress",
            "done",
        }

        if status not in allowed_statuses:
            raise HTTPException(
                status_code=400,
                detail=(
                    "Недопустимый статус. "
                    "Используйте: todo, in_progress или done"
                ),
            )

        query = query.filter(
            Task.status == status
        )

    # PRIORITY FILTER

    if priority is not None:

        allowed_priorities = {
            "low",
            "medium",
            "high",
        }

        if priority not in allowed_priorities:
            raise HTTPException(
                status_code=400,
                detail=(
                    "Недопустимый приоритет. "
                    "Используйте: low, medium или high"
                ),
            )

        query = query.filter(
            Task.priority == priority
        )

    # PAGINATION

    total = query.count()

    pages = ceil(total / limit) if total > 0 else 0

    offset = (page - 1) * limit

    tasks = (
        query
        .order_by(Task.id)
        .offset(offset)
        .limit(limit)
        .all()
    )

    items = [
        task_to_extended_response(
            task,
            include_set,
        )
        for task in tasks
    ]

    return {
        "items": items,
        "page": page,
        "limit": limit,
        "total": total,
        "pages": pages,
    }


# GET ONE TASK

@router.get(
    "/{task_id}",
    response_model=TaskV2Response,
)
def get_task(
    task_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """
    Получение одной задачи через API V2.
    """

    task = (
        db.query(Task)
        .join(Project)
        .filter(
            Task.id == task_id,
            Project.owner_id == current_user.id,
        )
        .first()
    )

    if not task:
        raise HTTPException(
            status_code=404,
            detail="Задача не найдена",
        )

    return task_to_v2_response(task)


# UPDATE TASK

@router.put(
    "/{task_id}",
    response_model=TaskV2Response,
)
def update_task(
    task_id: int,
    task_data: TaskCreate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """
    Обновление задачи через API V2.
    """

    task = (
        db.query(Task)
        .join(Project)
        .filter(
            Task.id == task_id,
            Project.owner_id == current_user.id,
        )
        .first()
    )

    if not task:
        raise HTTPException(
            status_code=404,
            detail="Задача не найдена",
        )

    project = db.query(Project).filter(
        Project.id == task_data.project_id,
        Project.owner_id == current_user.id,
    ).first()

    if not project:
        raise HTTPException(
            status_code=404,
            detail="Проект не найден или доступ запрещён",
        )

    if task_data.assignee_id is not None:

        assignee = db.query(User).filter(
            User.id == task_data.assignee_id
        ).first()

        if not assignee:
            raise HTTPException(
                status_code=404,
                detail="Пользователь-исполнитель не найден",
            )

    task.project_id = task_data.project_id
    task.title = task_data.title
    task.description = task_data.description
    task.status = task_data.status
    task.priority = task_data.priority
    task.assignee_id = task_data.assignee_id
    task.due_date = task_data.due_date

    db.commit()
    db.refresh(task)

    return task_to_v2_response(task)


# DELETE TASK

@router.delete(
    "/{task_id}",
    status_code=204,
)
def delete_task(
    task_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
):
    """
    Удаление задачи через API V2.
    """

    task = (
        db.query(Task)
        .join(Project)
        .filter(
            Task.id == task_id,
            Project.owner_id == current_user.id,
        )
        .first()
    )

    if not task:
        raise HTTPException(
            status_code=404,
            detail="Задача не найдена",
        )

    db.delete(task)
    db.commit()

    return None