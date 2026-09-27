let me = null;
let currentLectureId = null;

async function api(url, opts = {}) {
  const r = await fetch(url, { credentials: 'include', ...opts });
  if (!r.ok) {
    const text = await r.text();
    throw new Error(`${r.status}: ${text}`);
  }
  return r.json();
}

async function bootstrap() {
  try {
    me = await api('/api/login/me');
  } catch (e) {
    location.href = '/';
    return;
  }
  if (me.role !== 'teacher') {
    location.href = '/student';
    return;
  }
  document.getElementById('userName').textContent = me.name;
  await loadLectures();
}

async function loadLectures() {
  const items = await api('/api/lectures?owner_id=' + encodeURIComponent(me.id));
  const sel = document.getElementById('lectureSelect');
  sel.innerHTML = '';
  const list = items || [];
  if (!list.length) {
    const o = document.createElement('option');
    o.value = '';
    o.textContent = '— нет лекций —';
    o.disabled = true;
    sel.appendChild(o);
    return;
  }
  list.forEach(l => {
    const o = document.createElement('option');
    o.value = l.id;
    o.textContent = l.title;
    sel.appendChild(o);
  });
  currentLectureId = list[0].id;
  await loadQuestions(currentLectureId);

  sel.onchange = async () => {
    currentLectureId = sel.value;
    await loadQuestions(currentLectureId);
  };
}

async function loadQuestions(lectureId) {
  if (!lectureId) return;
  const qs = await api('/api/quiz-questions/' + lectureId);
  const box = document.getElementById('qList');
  box.innerHTML = '';
  const list = qs || [];
  if (!list.length) {
    box.innerHTML = '<p class="hint">Вопросов пока нет</p>';
    return;
  }
  list.forEach((q, i) => {
    const div = document.createElement('div');
    div.className = 'list';
    div.innerHTML = `
      <li>
        <div style="display:flex;justify-content:space-between;gap:12px">
          <strong>${i + 1}. ${q.text}</strong>
          <span class="del" data-id="${q.id}"
                style="cursor:pointer;color:var(--muted)">×</span>
        </div>
        <small>Правильный: ${q.options[q.correct]}</small>
        <small>Вариантов: ${q.options.length}</small>
      </li>`;
    div.querySelector('.del').onclick = async () => {
      await api('/api/quiz-questions/' + lectureId + '/' + q.id, { method: 'DELETE' })
        .catch(() => {});
      await loadQuestions(lectureId);
    };
    box.appendChild(div);
  });
}

document.getElementById('addQ').onclick = async () => {
  const lectureId = document.getElementById('lectureSelect').value;
  const text = document.getElementById('qText').value.trim();
  const options = [...document.querySelectorAll('.opt')]
    .map(i => i.value.trim())
    .filter(Boolean);
  const correct = parseInt(document.getElementById('correct').value, 10);

  if (!lectureId) return alert('Выберите лекцию');
  if (!text) return alert('Введите текст вопроса');
  if (options.length < 2) return alert('Нужно минимум 2 варианта');
  if (correct < 0 || correct >= options.length) return alert('Некорректный правильный вариант');

  try {
    await api('/api/quiz-questions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ lecture_id: lectureId, text, options, correct }),
    });
    document.getElementById('qText').value = '';
    document.querySelectorAll('.opt').forEach(i => i.value = '');
    await loadQuestions(lectureId);
  } catch (e) {
    alert('Ошибка: ' + e.message);
  }
};

bootstrap();
