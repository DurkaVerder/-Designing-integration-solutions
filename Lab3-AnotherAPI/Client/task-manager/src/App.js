import { useState } from 'react';
import { api } from './api';
import './App.css';

const emptyTask = { project_id: '', title: '', description: '', status: 'todo', priority: 'medium', assignee_id: '', due_date: '' };
const emptyProject = { name: '', description: '' };
const makeIdempotencyKey = (prefix) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
const savedProjectsKey = 'task-manager-projects';

function readSavedProjects() {
  try { return JSON.parse(localStorage.getItem(savedProjectsKey) || '[]'); } catch { return []; }
}

function saveProjects(projects) { localStorage.setItem(savedProjectsKey, JSON.stringify(projects)); }

function App() {
  const [token, setToken] = useState(localStorage.getItem('task-manager-token'));
  const [authMode, setAuthMode] = useState('login');
  const [auth, setAuth] = useState({ email: '', password: '', username: '' });
  const [projects, setProjects] = useState([]);
  const [selectedProject, setSelectedProject] = useState(null);
  const [selectedTask, setSelectedTask] = useState(null);
  const [tasks, setTasks] = useState([]);
  const [taskPage, setTaskPage] = useState({ page: 1, limit: 10, pages: 1, total: 0 });
  const [taskFilters, setTaskFilters] = useState({ status: '', priority: '' });
  const [taskForm, setTaskForm] = useState(emptyTask);
  const [projectForm, setProjectForm] = useState(emptyProject);
  const [taskIdempotencyKey, setTaskIdempotencyKey] = useState('');
  const [projectIdempotencyKey, setProjectIdempotencyKey] = useState('');
  const [editingTask, setEditingTask] = useState(false);
  const [editingProject, setEditingProject] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  function reportError(exception) { setError(exception.message || 'Произошла ошибка'); setMessage(''); }
  function reportSuccess(text) { setMessage(text); setError(''); }
  function taskPayload() { return { ...taskForm, project_id: Number(taskForm.project_id), assignee_id: taskForm.assignee_id ? Number(taskForm.assignee_id) : null, due_date: taskForm.due_date || null }; }

  // POST авторизации или регистрации. JWT сохраняется в localStorage.
  async function submitAuth(event) {
    event.preventDefault();
    try {
      const data = authMode === 'login' ? await api.login({ email: auth.email, password: auth.password }) : await api.register(auth);
      if (authMode === 'login') { localStorage.setItem('task-manager-token', data.access_token); setToken(data.access_token); await loadProjects(); }
      else { reportSuccess('Пользователь создан. Теперь войдите.'); setAuthMode('login'); }
    } catch (exception) { reportError(exception); }
  }

  // Отдельного GET /projects/ нет, поэтому проекты собираются из задач v2.
  async function loadProjects() {
    setLoading(true);
    try {
      const data = await api.listTasksV2({ page: 1, limit: 100, include: 'project' });
      const projectsFromTasks = data.items.filter((item) => item.project).map((item) => item.project);
      const uniqueProjects = Array.from(new Map([...projectsFromTasks, ...readSavedProjects()].map((project) => [project.id, project])).values());
      saveProjects(uniqueProjects);
      setProjects(uniqueProjects); reportSuccess('Проекты загружены');
    } catch (exception) { reportError(exception); } finally { setLoading(false); }
  }

  // Открывает проект и загружает только его задачи.
  async function openProject(project) {
    setSelectedProject(project); setSelectedTask(null); setLoading(true);
    try {
      await loadProjectTasks(project, 1);
      setTaskForm({ ...emptyTask, project_id: project.id });
    } catch (exception) { reportError(exception); } finally { setLoading(false); }
  }

  // Загружает полные данные выбранной задачи для страницы деталей.
  async function openTask(task) {
    setLoading(true);
    try { setSelectedTask(await api.getTaskV2(task.id)); } catch (exception) { reportError(exception); } finally { setLoading(false); }
  }

  // Повторно загружает текущую страницу задач после фильтрации или CRUD.
  async function reloadProject(page = taskPage.page) {
    if (!selectedProject) return;
    setLoading(true);
    try {
      await loadProjectTasks(selectedProject, page);
    } catch (exception) { reportError(exception); } finally { setLoading(false); }
  }

  // Backend v2 does not filter by project_id, so collect all API pages first.
  // Only then filter and paginate tasks belonging to the opened project.
  async function loadProjectTasks(project, page) {
    const limit = 100;
    const firstPage = await api.listTasksV2({ ...taskFilters, page: 1, limit, include: 'project,assignee' });
    let allTasks = firstPage.items;
    for (let currentPage = 2; currentPage <= firstPage.pages; currentPage += 1) {
      const nextPage = await api.listTasksV2({ ...taskFilters, page: currentPage, limit, include: 'project,assignee' });
      allTasks = allTasks.concat(nextPage.items);
    }

    const projectTasks = allTasks.filter((item) => item.project_id === project.id);
    const pages = Math.max(1, Math.ceil(projectTasks.length / taskPage.limit));
    const safePage = Math.min(page, pages);
    const start = (safePage - 1) * taskPage.limit;
    setTasks(projectTasks.slice(start, start + taskPage.limit));
    setTaskPage({ page: safePage, limit: taskPage.limit, pages, total: projectTasks.length });
  }

  // Создание или обновление задачи. V2 поддерживает идемпотентный ключ.
  async function submitTask(event) {
    event.preventDefault();
    try {
      const key = taskIdempotencyKey || makeIdempotencyKey('task');
      if (!editingTask) setTaskIdempotencyKey(key);
      const saved = editingTask ? await api.updateTaskV2(selectedTask.id, taskPayload()) : await api.createTaskV2(taskPayload(), key);
      if (!editingTask) setTaskIdempotencyKey('');
      reportSuccess(editingTask ? 'Задача обновлена' : 'Задача создана'); setEditingTask(false); setSelectedTask(editingTask ? saved : null); await reloadProject();
    } catch (exception) { reportError(exception); }
  }

  // Удаление задачи из страницы деталей.
  async function deleteTask() {
    try { await api.deleteTaskV2(selectedTask.id); reportSuccess('Задача удалена'); setSelectedTask(null); await reloadProject(); } catch (exception) { reportError(exception); }
  }

  // Создание или обновление проекта.
  async function submitProject(event) {
    event.preventDefault();
    try {
      const key = projectIdempotencyKey || makeIdempotencyKey('project');
      if (!editingProject) setProjectIdempotencyKey(key);
      const data = editingProject ? await api.updateProject(selectedProject.id, projectForm) : await api.createProject(projectForm, key);
      if (!editingProject) setProjectIdempotencyKey('');
      const saved = { id: data.id, name: data.name, description: data.description };
      const nextProjects = editingProject ? projects.map((item) => item.id === saved.id ? saved : item) : [...projects, saved];
      saveProjects(nextProjects);
      setProjects(nextProjects);
      if (editingProject) setSelectedProject(saved);
      setProjectForm(emptyProject); setEditingProject(false); reportSuccess(editingProject ? 'Проект обновлён' : 'Проект создан');
    } catch (exception) { reportError(exception); }
  }

  // Удаление открытого проекта.
  async function deleteProject() {
    try {
      await api.deleteProject(selectedProject.id);
      const nextProjects = projects.filter((item) => item.id !== selectedProject.id);
      saveProjects(nextProjects);
      setProjects(nextProjects); setSelectedProject(null); reportSuccess('Проект удалён');
    } catch (exception) { reportError(exception); }
  }

  // Заполняет и открывает модальное окно редактирования проекта.
  async function editProject() {
    setProjectForm({ name: selectedProject.name, description: selectedProject.description || '' });
    setEditingProject(true);
  }

  // Заполняет и открывает модальное окно редактирования задачи.
  function editTaskForm() {
    setTaskForm({ ...selectedTask, project_id: selectedTask.project_id, assignee_id: selectedTask.assignee_id || '', due_date: selectedTask.due_date ? selectedTask.due_date.slice(0, 16) : '' });
    setEditingTask(true);
  }

  function logout() { localStorage.removeItem('task-manager-token'); setToken(null); setProjects([]); setSelectedProject(null); setSelectedTask(null); }

  if (!token) return <AuthScreen mode={authMode} auth={auth} setAuth={setAuth} setMode={setAuthMode} onSubmit={submitAuth} error={error} message={message} />;
  return <div className="app"><header className="topbar"><button className="brand" onClick={() => { setSelectedProject(null); setSelectedTask(null); }}>Task manager</button><span>Рабочее пространство</span><button onClick={logout}>Выйти</button></header><main className="container">{error && <div className="alert error">{error}</div>}{message && <div className="alert success">{message}</div>}{!selectedProject && !selectedTask && <ProjectList projects={projects} loading={loading} onLoad={loadProjects} onSelect={openProject} form={projectForm} setForm={setProjectForm} idempotencyKey={projectIdempotencyKey} setIdempotencyKey={setProjectIdempotencyKey} onSubmit={submitProject} />}{selectedProject && !selectedTask && <TaskList project={selectedProject} tasks={tasks} loading={loading} page={taskPage} filters={taskFilters} setFilters={setTaskFilters} onBack={() => setSelectedProject(null)} onRefresh={() => reloadProject(1)} onSelect={openTask} onPage={reloadProject} form={taskForm} setForm={setTaskForm} idempotencyKey={taskIdempotencyKey} setIdempotencyKey={setTaskIdempotencyKey} onSubmit={submitTask} onEditProject={editProject} onDeleteProject={deleteProject} />}{selectedTask && <TaskDetails task={selectedTask} onBack={() => setSelectedTask(null)} onEdit={editTaskForm} onDelete={deleteTask} />}{editingProject && <ProjectModal form={projectForm} setForm={setProjectForm} onSubmit={submitProject} onClose={() => { setEditingProject(false); setProjectForm(emptyProject); }} />}{editingTask && selectedTask && <TaskModal form={taskForm} setForm={setTaskForm} onSubmit={submitTask} onClose={() => { setEditingTask(false); setTaskForm(emptyTask); }} />}</main></div>;
}

function AuthScreen({ mode, auth, setAuth, setMode, onSubmit, error, message }) { return <div className="auth-page"><form className="auth-card" onSubmit={onSubmit}><h1>Task manager</h1><p>{mode === 'login' ? 'Войдите, чтобы открыть проекты' : 'Создайте аккаунт'}</p>{error && <div className="alert error">{error}</div>}{message && <div className="alert success">{message}</div>}{mode === 'register' && <input name="username" placeholder="Имя пользователя" value={auth.username} onChange={(event) => setAuth({ ...auth, username: event.target.value })} required />}<input name="email" type="email" placeholder="Email" value={auth.email} onChange={(event) => setAuth({ ...auth, email: event.target.value })} required /><input name="password" type="password" placeholder="Пароль" value={auth.password} onChange={(event) => setAuth({ ...auth, password: event.target.value })} required /><button type="submit">{mode === 'login' ? 'Войти' : 'Зарегистрироваться'}</button><button type="button" className="text-button" onClick={() => setMode(mode === 'login' ? 'register' : 'login')}>{mode === 'login' ? 'Создать аккаунт' : 'Вернуться ко входу'}</button></form></div>; }

function ProjectList({ projects, loading, onLoad, onSelect, form, setForm, idempotencyKey, setIdempotencyKey, onSubmit }) { return <section><div className="page-heading"><div><h1>Проекты</h1><p>Выберите проект, чтобы открыть его задачи.</p></div><button onClick={onLoad}>{loading ? 'Загрузка...' : 'Обновить'}</button></div><form className="inline-form" onSubmit={onSubmit}><input name="name" placeholder="Название нового проекта" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} required /><input name="description" placeholder="Описание" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /><input className="key-input" placeholder="Idempotency-Key (необязательно)" value={idempotencyKey} onChange={(event) => setIdempotencyKey(event.target.value)} /><button type="submit">Создать</button></form>{projects.length === 0 ? <div className="empty">Проекты пока не загружены. Нажмите «Обновить».</div> : <div className="project-list">{projects.map((project) => <button className="project-row" key={project.id} onClick={() => onSelect(project)}><span><strong>{project.name}</strong><small>{project.description || 'Без описания'}</small></span><span>Открыть →</span></button>)}</div>}</section>; }

function TaskList({ project, tasks, loading, page, filters, setFilters, onBack, onRefresh, onSelect, onPage, form, setForm, idempotencyKey, setIdempotencyKey, onSubmit, onEditProject, onDeleteProject }) { return <section><div className="breadcrumbs"><button onClick={onBack}>Проекты</button><span>/</span><strong>{project.name}</strong></div><div className="page-heading"><div><h1>{project.name}</h1><p>{project.description || 'Задачи проекта'}</p></div><div className="actions"><button onClick={onEditProject}>Изменить проект</button><button className="danger" onClick={onDeleteProject}>Удалить проект</button></div></div><div className="toolbar"><select value={filters.status} onChange={(event) => setFilters({ ...filters, status: event.target.value })}><option value="">Все статусы</option><option value="todo">К выполнению</option><option value="in_progress">В работе</option><option value="done">Готово</option></select><select value={filters.priority} onChange={(event) => setFilters({ ...filters, priority: event.target.value })}><option value="">Все приоритеты</option><option value="low">Низкий</option><option value="medium">Средний</option><option value="high">Высокий</option></select><button onClick={onRefresh}>Применить</button></div><div className="task-layout"><div className="task-column"><h2>Задачи <span>{page.total}</span></h2>{loading ? <div className="empty">Загрузка...</div> : tasks.length === 0 ? <div className="empty">В этом проекте нет задач.</div> : <div className="task-list">{tasks.map((task) => <button className="task-row" key={task.id} onClick={() => onSelect(task)}><span><strong>{task.title}</strong><small>{task.description || 'Без описания'}</small></span><span className={`status ${task.status}`}>{task.status}</span></button>)}</div>}<div className="pagination"><button disabled={page.page <= 1} onClick={() => onPage(page.page - 1)}>Назад</button><span>Страница {page.page} из {page.pages}</span><button disabled={page.page >= page.pages} onClick={() => onPage(page.page + 1)}>Вперёд</button></div></div><form className="task-form" onSubmit={onSubmit}><h2>Новая задача</h2><input name="title" placeholder="Название" value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} required /><textarea name="description" placeholder="Описание" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /><select name="status" value={form.status} onChange={(event) => setForm({ ...form, status: event.target.value })}><option value="todo">К выполнению</option><option value="in_progress">В работе</option><option value="done">Готово</option></select><select name="priority" value={form.priority} onChange={(event) => setForm({ ...form, priority: event.target.value })}><option value="low">Низкий</option><option value="medium">Средний</option><option value="high">Высокий</option></select><input name="assignee_id" placeholder="ID исполнителя" value={form.assignee_id} onChange={(event) => setForm({ ...form, assignee_id: event.target.value })} /><input name="due_date" type="datetime-local" value={form.due_date} onChange={(event) => setForm({ ...form, due_date: event.target.value })} /><input className="key-input" placeholder="Idempotency-Key (необязательно)" value={idempotencyKey} onChange={(event) => setIdempotencyKey(event.target.value)} /><button type="submit">Создать задачу</button></form></div></section>; }

function TaskDetails({ task, onBack, onEdit, onDelete }) { return <section><div className="breadcrumbs"><button onClick={onBack}>К задачам</button><span>/</span><strong>Задача #{task.id}</strong></div><article className="details"><div className="details-heading"><div><h1>{task.title}</h1><p>{task.description || 'Описание отсутствует'}</p></div><div className="actions"><button onClick={onEdit}>Изменить</button><button className="danger" onClick={onDelete}>Удалить</button></div></div><dl><div><dt>Статус</dt><dd>{task.status}</dd></div><div><dt>Приоритет</dt><dd>{task.priority}</dd></div><div><dt>Проект</dt><dd>{task.project ? task.project.name : `#${task.project_id}`}</dd></div><div><dt>Исполнитель</dt><dd>{task.assignee ? task.assignee.username : task.assignee_id || 'Не назначен'}</dd></div><div><dt>Срок</dt><dd>{task.due_date || 'Не указан'}</dd></div><div><dt>Создана</dt><dd>{task.created_at || '—'}</dd></div></dl>{typeof task.is_overdue === 'boolean' && <p className={task.is_overdue ? 'overdue' : 'on-time'}>{task.is_overdue ? 'Задача просрочена' : 'Срок не нарушен'}</p>}</article></section>; }

function ProjectModal({ form, setForm, onSubmit, onClose }) {
  return <div className="modal-backdrop" onMouseDown={onClose}><form className="modal" onSubmit={onSubmit} onMouseDown={(event) => event.stopPropagation()}><div className="modal-header"><h2>Изменить проект</h2><button type="button" className="close-button" onClick={onClose}>×</button></div><label>Название<input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} required /></label><label>Описание<textarea value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /></label><div className="modal-actions"><button type="button" onClick={onClose}>Отмена</button><button type="submit">Сохранить</button></div></form></div>;
}

function TaskModal({ form, setForm, onSubmit, onClose }) {
  return <div className="modal-backdrop" onMouseDown={onClose}><form className="modal" onSubmit={onSubmit} onMouseDown={(event) => event.stopPropagation()}><div className="modal-header"><h2>Изменить задачу</h2><button type="button" className="close-button" onClick={onClose}>×</button></div><label>Название<input value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} required /></label><label>Описание<textarea value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} /></label><label>Статус<select value={form.status} onChange={(event) => setForm({ ...form, status: event.target.value })}><option value="todo">К выполнению</option><option value="in_progress">В работе</option><option value="done">Готово</option></select></label><label>Приоритет<select value={form.priority} onChange={(event) => setForm({ ...form, priority: event.target.value })}><option value="low">Низкий</option><option value="medium">Средний</option><option value="high">Высокий</option></select></label><label>Назначенный пользователь<input type="number" min="1" placeholder="ID пользователя" value={form.assignee_id} onChange={(event) => setForm({ ...form, assignee_id: event.target.value })} /></label><div className="modal-actions"><button type="button" onClick={onClose}>Отмена</button><button type="submit">Сохранить</button></div></form></div>;
}

export default App;