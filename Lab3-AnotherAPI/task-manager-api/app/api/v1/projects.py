from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.database import get_db
from app.models import Project, User
from app.schemas import ProjectCreate, ProjectResponse
from app.dependencies import get_current_user

import json

from fastapi import Header
from fastapi.responses import JSONResponse

from app.idempotency import (
    get_idempotency_record,
    save_idempotency_record
)

router = APIRouter(
    prefix="/projects",
    tags=["Projects"]
)


@router.post("/", response_model=ProjectResponse, status_code=201)
def create_project(
    project_data: ProjectCreate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user),
    idempotency_key: str | None = Header(
        default=None,
        alias="Idempotency-Key"
    )
):
    if not idempotency_key:
        raise HTTPException(
            status_code=400,
            detail="Необходимо передать заголовок Idempotency-Key"
        )

    endpoint = "/api/v1/projects/"

    # Проверяем, использовался ли ключ ранее
    existing_record = get_idempotency_record(
        db,
        idempotency_key,
        endpoint
    )

    if existing_record:
        return JSONResponse(
            status_code=existing_record.response_status,
            content=json.loads(existing_record.response_body)
        )

    # Создаём проект
    project = Project(
        name=project_data.name,
        description=project_data.description,
        owner_id=current_user.id
    )

    db.add(project)
    db.commit()
    db.refresh(project)

    response_data = {
        "id": project.id,
        "name": project.name,
        "description": project.description,
        "owner_id": project.owner_id,
        "created_at": project.created_at
    }

    # Сохраняем результат запроса
    save_idempotency_record(
        db,
        idempotency_key,
        endpoint,
        201,
        response_data
    )

    return project


@router.get("/{project_id}", response_model=ProjectResponse)
def get_project(
    project_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    project = db.query(Project).filter(
        Project.id == project_id,
        Project.owner_id == current_user.id
    ).first()

    if not project:
        raise HTTPException(
            status_code=404,
            detail="Проект не найден"
        )

    return project  


@router.put("/{project_id}", response_model=ProjectResponse)
def update_project(
    project_id: int,
    project_data: ProjectCreate,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    project = db.query(Project).filter(
        Project.id == project_id,
        Project.owner_id == current_user.id
    ).first()

    if not project:
        raise HTTPException(
            status_code=404,
            detail="Проект не найден"
        )

    project.name = project_data.name
    project.description = project_data.description

    db.commit()
    db.refresh(project)

    return project


@router.delete("/{project_id}", status_code=204)
def delete_project(
    project_id: int,
    db: Session = Depends(get_db),
    current_user: User = Depends(get_current_user)
):
    project = db.query(Project).filter(
        Project.id == project_id,
        Project.owner_id == current_user.id
    ).first()

    if not project:
        raise HTTPException(
            status_code=404,
            detail="Проект не найден"
        )

    db.delete(project)
    db.commit()

    return None