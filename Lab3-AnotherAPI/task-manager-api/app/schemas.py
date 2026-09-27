from datetime import datetime

from pydantic import BaseModel, ConfigDict, EmailStr


class UserCreate(BaseModel):
    username: str
    email: EmailStr
    password: str


class LoginRequest(BaseModel):
    email: EmailStr
    password: str


class UserResponse(BaseModel):
    id: int
    username: str
    email: EmailStr
    created_at: datetime

    model_config = ConfigDict(from_attributes=True)


class ProjectCreate(BaseModel):
    name: str
    description: str | None = None


class ProjectResponse(BaseModel):
    id: int
    name: str
    description: str | None
    owner_id: int
    created_at: datetime

    model_config = ConfigDict(from_attributes=True)


class TaskCreate(BaseModel):
    project_id: int
    title: str
    description: str | None = None
    status: str = "todo"
    priority: str = "medium"
    assignee_id: int | None = None
    due_date: datetime | None = None


class TaskResponse(BaseModel):
    id: int
    project_id: int
    title: str
    description: str | None
    status: str
    priority: str
    assignee_id: int | None
    created_at: datetime
    due_date: datetime | None

    model_config = ConfigDict(from_attributes=True)


class TaskV2Response(BaseModel):
    id: int
    project_id: int
    title: str
    description: str | None
    status: str
    priority: str
    assignee_id: int | None
    created_at: datetime
    due_date: datetime | None

    # Дополнительные возможности V2
    is_overdue: bool
    days_until_due: int | None

    model_config = ConfigDict(from_attributes=True)

# V2: дополнительные схемы

class TaskProjectShort(BaseModel):
    id: int
    name: str

    model_config = ConfigDict(from_attributes=True)


class TaskAssigneeShort(BaseModel):
    id: int
    username: str

    model_config = ConfigDict(from_attributes=True)


class TaskV2ExtendedResponse(TaskV2Response):
    """
    Расширенная схема V2.

    Дополнительные поля project и assignee
    возвращаются только при использовании include.
    """

    project: TaskProjectShort | None = None
    assignee: TaskAssigneeShort | None = None


class TaskPaginationResponse(BaseModel):
    """
    Пагинированный ответ API V2.
    """

    items: list[TaskV2ExtendedResponse]
    page: int
    limit: int
    total: int
    pages: int