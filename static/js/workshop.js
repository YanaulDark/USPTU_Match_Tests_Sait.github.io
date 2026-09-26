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
  console.log('[workshop] bootstrapping...');
  try {
    me = await api('/api/login/me');
    console.log('[workshop] me =', me);
  } catch (e) {
    console.error('[workshop] not authenticated:', e.message);
    location.href = '/';
    return;
  }

  try {
    await loadLectures();
  } catch (e) {
    console.error('[workshop] loadLectures failed:', e.message);
  }
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
  const qs = await api('/api/questions/' + lectureId);
  const box = document.getElementById('qList');
  box.innerHTML = '';
  const list = qs || [];
  if (!list.length) {
    box.innerHTML = '<p class="hint">Вопросов пока нет</p>';
    return;
  }
  list.forEach(q => {
    const div = document.createElement('div');
    div.className = 'list';
    div.innerHTML = `<li><strong>Слайд ${q.slide}: ${q.text}</strong>
      <small>Правильный: ${q.options[q.correct]}</small></li>`;
    box.appendChild(div);
  });
}

document.getElementById('addQ').onclick = async () => {
  const lectureId = document.getElementById('lectureSelect').value || currentLectureId;
  const slide = parseInt(document.getElementById('slideNum').value, 10);
  const text = document.getElementById('qText').value.trim();
  const options = [...document.querySelectorAll('.opt')]
    .map(i => i.value.trim())
    .filter(Boolean);
  const correct = parseInt(document.getElementById('correct').value, 10);

  if (!lectureId) return alert('Выберите лекцию');
  if (!text) return alert('Введите текст вопроса');
  if (options.length < 2) return alert('Нужно минимум 2 варианта ответа');
  if (correct < 0 || correct >= options.length) return alert('Некорректный правильный вариант');

  try {
    await api('/api/questions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        lecture_id: lectureId,
        slide,
        text,
        options,
        correct,
      }),
    });
    // очистить форму
    document.getElementById('qText').value = '';
    document.querySelectorAll('.opt').forEach(i => i.value = '');
    await loadQuestions(lectureId);
  } catch (e) {
    alert('Не удалось добавить вопрос: ' + e.message);
  }
};

bootstrap();

