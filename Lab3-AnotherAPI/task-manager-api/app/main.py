from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.v1 import users, projects, tasks, auth
from app.api.v2 import tasks as tasks_v2
from app.api.internal import statistics
from app.rate_limit import rate_limit_middleware


app = FastAPI(
    title="Task Manager API",
    description="REST API для системы управления задачами",
    version="2.0.0"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:3000",
        "http://127.0.0.1:3000",
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# Rate Limiting
app.middleware("http")(rate_limit_middleware)


# API V1

app.include_router(
    auth.router,
    prefix="/api/v1"
)

app.include_router(
    users.router,
    prefix="/api/v1"
)

app.include_router(
    projects.router,
    prefix="/api/v1"
)

app.include_router(
    tasks.router,
    prefix="/api/v1"
)

# API V2

app.include_router(
    tasks_v2.router,
    prefix="/api/v2"
)

app.include_router(
    statistics.router,
    prefix="/api/internal"
)


@app.get("/")
def root():
    return {
        "message": "Task Manager API is running"
    }