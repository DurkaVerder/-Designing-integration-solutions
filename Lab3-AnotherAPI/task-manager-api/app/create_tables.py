from app.database import Base, engine
from app import models


print("Создание таблиц...")

Base.metadata.create_all(bind=engine)

print("Таблицы успешно созданы!")