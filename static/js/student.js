let me = null;
let currentTeacherId = null;
let currentTeacherName = '';
let currentLectureId = null;

async function api(url, opts = {}) {
  const r = await fetch(url, { credentials: 'include', ...opts });
  if (!r.ok) {
    const text = await r.text();
    throw new Error(`${r.status}: ${text}`);
  }
  return r.json();
}

// ─── Bootstrap ───

async function bootstrap() {
  try {
    me = await api('/api/login/me');
  } catch (e) {
    location.href = '/';
    return;
  }
  if (me.role !== 'student') {
    location.href = '/teacher';
    return;
  }
  document.getElementById('userName').textContent = me.name;
  await loadFriends();
  await loadTeachers();
}

// ─── Друзья ───

async function loadFriends() {
  const items = await api('/api/friends');
  const ul = document.getElementById('friendsList');
  ul.innerHTML = '';
  if (!items || !items.length) {
    ul.innerHTML = '<li class="hint">Пока никого</li>';
    return;
  }
  items.forEach(f => {
    const li = document.createElement('li');
    li.innerHTML = `<span>${f.name}</span>
      <span class="del" title="Удалить">×</span>`;
    li.querySelector('.del').onclick = async (e) => {
      e.stopPropagation();
      await api('/api/friends/' + f.id, { method: 'DELETE' }).catch(() => {});
      await loadFriends();
    };
    ul.appendChild(li);
  });
}

// ─── Преподаватели ───

async function loadTeachers() {
  const items = await api('/api/subscriptions');
  const ul = document.getElementById('teachersList');
  ul.innerHTML = '';
  if (!items || !items.length) {
    ul.innerHTML = '<li class="hint">Пока никого</li>';
    return;
  }
  items.forEach(t => {
    const li = document.createElement('li');
    li.dataset.id = t.id;
    li.innerHTML = `<span>${t.name}</span>
      <span class="del" title="Отписаться">×</span>`;
    li.onclick = () => openTeacher(t.id, t.name, li);
    li.querySelector('.del').onclick = async (e) => {
      e.stopPropagation();
      await api('/api/subscriptions/' + t.id, { method: 'DELETE' }).catch(() => {});
      if (currentTeacherId === t.id) {
        currentTeacherId = null;
        showEmptyState();
      }
      await loadTeachers();
    };
    ul.appendChild(li);
  });
}

// ─── Открыть преподавателя → список лекций ───

async function openTeacher(id, name, liEl) {
  currentTeacherId = id;
  currentTeacherName = name;
  currentLectureId = null;

  document.querySelectorAll('#teachersList li').forEach(x => x.classList.remove('active'));
  if (liEl) liEl.classList.add('active');

  const lectures = await api('/api/teachers/' + id + '/lectures');
  renderLectures(name, lectures || []);
}

function renderLectures(teacherName, lectures) {
  const content = document.getElementById('studentContent');
  if (!lectures.length) {
    content.innerHTML = `
      <div class="section-head">
        <h2>${teacherName}</h2>
      </div>
      <p class="hint">У этого преподавателя пока нет лекций</p>`;
    return;
  }
  content.innerHTML = `
    <div class="section-head">
      <h2>Лекции: ${teacherName}</h2>
    </div>
    <div class="lectures-grid" id="lecturesGrid"></div>`;

  const grid = document.getElementById('lecturesGrid');
  lectures.forEach(l => {
    const card = document.createElement('div');
    card.className = 'lecture-card';
    card.innerHTML = `<h4>${l.title}</h4>
      <small>${new Date(l.created_at).toLocaleDateString('ru-RU')}</small>`;
    card.onclick = () => openLecture(l);
    grid.appendChild(card);
  });
}

// ─── Открыть лекцию → вопросы ───

async function openLecture(l) {
  currentLectureId = l.id;
  const data = await api('/api/lectures/' + l.id).catch(() => null);
  // /api/lectures/<id> под AuthMiddleware(...) без роли — работает для студента
  const questions = (data && data.questions) || [];

  const content = document.getElementById('studentContent');
  content.innerHTML = `
    <div class="section-head">
      <button class="btn back" id="backBtn">← К лекциям</button>
      <h2>${l.title}</h2>
    </div>
    <div id="questionList"></div>`;

  document.getElementById('backBtn').onclick = () => {
    openTeacher(currentTeacherId, currentTeacherName);
  };

  const box = document.getElementById('questionList');
  if (!questions.length) {
    box.innerHTML = '<p class="hint">К этой лекции пока нет вопросов</p>';
    return;
  }
  questions.forEach(q => box.appendChild(renderQuestion(q)));
}

function renderQuestion(q) {
  const card = document.createElement('div');
  card.className = 'question-card';
  card.innerHTML = `<h3>Слайд ${q.slide}: ${q.text}</h3>`;
  q.options.forEach((opt, i) => {
    const b = document.createElement('button');
    b.className = 'option';
    b.textContent = opt;
    b.onclick = async () => {
      try {
        const res = await api('/api/answers?lecture_id=' + currentLectureId, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            question_id: q.id,
            student_id: me.id,
            choice: i,
          }),
        });
        card.querySelectorAll('.option').forEach((o, j) => {
          o.disabled = true;
          if (j === q.correct) o.classList.add('correct');
          else if (j === i) o.classList.add('wrong');
        });
      } catch (e) {
        alert('Не удалось отправить ответ: ' + e.message);
      }
    };
    card.appendChild(b);
  });
  return card;
}

function showEmptyState() {
  document.getElementById('studentContent').innerHTML = `
    <div class="empty-state">
      <h2>Выберите преподавателя</h2>
      <p class="hint">Слева выберите преподавателя, чтобы увидеть его лекции</p>
    </div>`;
}

// ─── Модалка добавления ───

let modalMode = null; // 'friend' | 'teacher'

function openModal(mode) {
  modalMode = mode;
  document.getElementById('modalTitle').textContent =
    mode === 'friend' ? 'Добавить друга' : 'Подписаться на преподавателя';
  document.getElementById('modalEmail').value = '';
  document.getElementById('modalMsg').textContent = '';
  document.getElementById('addModal').classList.remove('hidden');
}

function closeModal() {
  document.getElementById('addModal').classList.add('hidden');
}

document.getElementById('addFriendBtn').onclick = () => openModal('friend');
document.getElementById('addTeacherBtn').onclick = () => openModal('teacher');
document.getElementById('modalCancel').onclick = closeModal;

document.getElementById('modalOk').onclick = async () => {
  const email = document.getElementById('modalEmail').value.trim();
  const msg = document.getElementById('modalMsg');
  if (!email) {
    msg.textContent = 'Введите email';
    return;
  }
  const url = modalMode === 'friend' ? '/api/friends' : '/api/subscriptions';
  try {
    await api(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    });
    closeModal();
    if (modalMode === 'friend') await loadFriends();
    else await loadTeachers();
  } catch (e) {
    msg.textContent = e.message;
  }
};

// закрытие кликом по фону
document.getElementById('addModal').onclick = (e) => {
  if (e.target.id === 'addModal') closeModal();
};

bootstrap();
