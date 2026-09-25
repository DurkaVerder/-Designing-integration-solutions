const API_URL = "http://localhost:8080/api/v2";
const tokenKey = "todo-list-token";
const userKey = "todo-list-user";

const authView = document.querySelector("#auth-view");
const tasksView = document.querySelector("#tasks-view");
const authMessage = document.querySelector("#auth-message");
const taskMessage = document.querySelector("#task-message");
const loginForm = document.querySelector("#login-form");
const registerForm = document.querySelector("#register-form");
const taskForm = document.querySelector("#task-form");
const tasksList = document.querySelector("#tasks-list");
let editingTaskId = null;

const statusLabels = { new: "Новая", in_progress: "В работе", done: "Готово" };
const priorityLabels = { low: "Низкий", medium: "Средний", high: "Высокий" };

function getToken() { return localStorage.getItem(tokenKey); }
function getUser() {
  try { return JSON.parse(localStorage.getItem(userKey) || "null"); } catch { return null; }
}
function showMessage(element, message = "") { element.textContent = message; }
function setAuthenticated(isAuthenticated) {
  authView.classList.toggle("is-hidden", isAuthenticated);
  tasksView.classList.toggle("is-hidden", !isAuthenticated);
  if (isAuthenticated) {
    const user = getUser();
    document.querySelector("#welcome-title").textContent = user ? `Привет, ${user.username}` : "Мои задачи";
    loadTasks();
  }
}

async function request(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  if (getToken()) headers.Authorization = `Bearer ${getToken()}`;
  const response = await fetch(`${API_URL}${path}`, { ...options, headers });
  if (response.status === 204) return null;
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `Ошибка запроса (${response.status})`);
  return body;
}

function formData(form) { return Object.fromEntries(new FormData(form).entries()); }

async function authenticate(event, isRegistration) {
  event.preventDefault();
  showMessage(authMessage);
  const form = event.currentTarget;
  try {
    const result = await request(isRegistration ? "/auth/register" : "/auth/login", {
      method: "POST",
      body: JSON.stringify(formData(form)),
    });
    if (isRegistration) {
      loginForm.elements.email.value = form.elements.email.value;
      switchAuthTab("login");
      showMessage(authMessage, "Аккаунт создан. Теперь войдите.");
      return;
    }
    localStorage.setItem(tokenKey, result.token);
    localStorage.setItem(userKey, JSON.stringify(result.user));
    form.reset();
    setAuthenticated(true);
  } catch (error) { showMessage(authMessage, error.message); }
}

function switchAuthTab(tabName) {
  document.querySelectorAll("[data-auth-tab]").forEach((tab) => tab.classList.toggle("is-active", tab.dataset.authTab === tabName));
  loginForm.classList.toggle("is-hidden", tabName !== "login");
  registerForm.classList.toggle("is-hidden", tabName !== "register");
  showMessage(authMessage);
}

function renderTasks(tasks) {
  if (!tasks.length) {
    tasksList.innerHTML = '<p class="empty-state">Задач пока нет. Добавьте первую!</p>';
    return;
  }
  tasksList.innerHTML = tasks.map((task) => `
    <article class="task-card ${task.status === "done" ? "is-done" : ""}">
      <div class="task-card-header">
        <h3 class="task-title">${escapeHtml(task.title)}</h3>
        <span class="badge badge-status">${statusLabels[task.status] || task.status}</span>
      </div>
      ${task.description ? `<p class="task-description">${escapeHtml(task.description)}</p>` : ""}
      <div class="task-meta">
        <span class="badge badge-${task.priority || "medium"}">${priorityLabels[task.priority] || "Средний"}</span>
        <span>${task.id}</span>
      </div>
      <div class="task-actions">
        <button data-action="edit" data-id="${task.id}" type="button">Изменить</button>
        <button class="delete" data-action="delete" data-id="${task.id}" type="button">Удалить</button>
      </div>
    </article>
  `).join("");
  tasksList._tasks = tasks;
}

function escapeHtml(value) {
  return String(value || "").replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" }[char]));
}

async function loadTasks() {
  tasksList.innerHTML = '<p class="empty-state">Загрузка задач...</p>';
  try { renderTasks(await request("/tasks")); }
  catch (error) {
    if (error.message.includes("401")) logout();
    else tasksList.innerHTML = `<p class="empty-state">${escapeHtml(error.message)}</p>`;
  }
}

async function saveTask(event) {
  event.preventDefault();
  showMessage(taskMessage);
  const data = formData(taskForm);
  try {
    await request(editingTaskId ? `/tasks/${editingTaskId}` : "/tasks", {
      method: editingTaskId ? "PUT" : "POST",
      headers: editingTaskId ? {} : { "Idempotency-Key": crypto.randomUUID() },
      body: JSON.stringify(data),
    });
    resetTaskForm();
    await loadTasks();
  } catch (error) { showMessage(taskMessage, error.message); }
}

function startEditing(task) {
  editingTaskId = task.id;
  Object.entries(task).forEach(([key, value]) => { if (taskForm.elements[key]) taskForm.elements[key].value = value || ""; });
  document.querySelector("#form-title").textContent = "Изменить задачу";
  document.querySelector("#save-task").textContent = "Сохранить";
  document.querySelector("#cancel-edit").classList.remove("is-hidden");
  taskForm.scrollIntoView({ behavior: "smooth", block: "start" });
}

function resetTaskForm() {
  editingTaskId = null;
  taskForm.reset();
  taskForm.elements.priority.value = "medium";
  document.querySelector("#form-title").textContent = "Добавить задачу";
  document.querySelector("#save-task").textContent = "Добавить задачу";
  document.querySelector("#cancel-edit").classList.add("is-hidden");
  showMessage(taskMessage);
}

async function handleTaskAction(event) {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  const task = (tasksList._tasks || []).find((item) => item.id === button.dataset.id);
  if (!task) return;
  if (button.dataset.action === "edit") { startEditing(task); return; }
  if (!confirm("Удалить эту задачу?")) return;
  try { await request(`/tasks/${task.id}`, { method: "DELETE" }); await loadTasks(); }
  catch (error) { showMessage(taskMessage, error.message); }
}

function logout() {
  localStorage.removeItem(tokenKey);
  localStorage.removeItem(userKey);
  resetTaskForm();
  setAuthenticated(false);
}

document.querySelectorAll("[data-auth-tab]").forEach((tab) => tab.addEventListener("click", () => switchAuthTab(tab.dataset.authTab)));
loginForm.addEventListener("submit", (event) => authenticate(event, false));
registerForm.addEventListener("submit", (event) => authenticate(event, true));
taskForm.addEventListener("submit", saveTask);
tasksList.addEventListener("click", handleTaskAction);
document.querySelector("#cancel-edit").addEventListener("click", resetTaskForm);
document.querySelector("#refresh-button").addEventListener("click", loadTasks);
document.querySelector("#logout-button").addEventListener("click", logout);
setAuthenticated(Boolean(getToken()));
