// ─── Константы ───
pdfjsLib.GlobalWorkerOptions.workerSrc = "/static/js/pdf.worker.min.js";

// ─── Состояние ───
let me = null;
let currentLecture = null;
let pdfDoc = null;
let pageNum = 1;
let questions = [];
let quizActive = false;
let statsInterval = null;

// ─── Утилиты ───
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
  console.log('[teacher] bootstrapping...');
  try {
    me = await api('/api/login/me');
    console.log('[teacher] me =', me);
  } catch (e) {
    console.error('[teacher] not authenticated:', e.message);
    location.href = '/';
    return;
  }

  const nameEl = document.getElementById('userName');
  if (nameEl) nameEl.textContent = me.name;

  try {
    await loadLectures();
  } catch (e) {
    console.error('[teacher] loadLectures failed:', e.message);
  }
}

// ─── Список лекций ───
async function loadLectures() {
  const items = await api('/api/lectures?owner_id=' + encodeURIComponent(me.id));
  const ul = document.getElementById('lectureList');
  if (!ul) return;
  ul.innerHTML = '';
  const list = items || [];
  if (!list.length) {
    ul.innerHTML = '<li style="opacity:.5;cursor:default">Пока нет лекций</li>';
    return;
  }
  list.forEach(l => {
    const li = document.createElement('li');
    li.textContent = l.title || l.id;
    li.onclick = () => selectLecture(l);
    ul.appendChild(li);
  });
}

// ─── Выбор лекции ───
async function selectLecture(l) {
  currentLecture = l;
  document.getElementById('lectureTitle').textContent = l.title;

  const data = await api('/api/lectures/' + l.id);
  questions = data.questions || [];
  renderQuestions();

  await loadPDF(l.pdf_path);

  // сброс квиз-UI
  syncQuizUI();
}

// ─── PDF ───
async function loadPDF(url) {
  const task = pdfjsLib.getDocument(url);
  pdfDoc = await task.promise;
  pageNum = 1;
  await renderPage();
}

let currentRenderTask = null;

async function renderPage() {
  // Отменяем предыдущий рендер, если он ещё идёт
  if (currentRenderTask) {
    try { currentRenderTask.cancel(); } catch (e) { /* ignore */ }
    currentRenderTask = null;
  }

  const page = await pdfDoc.getPage(pageNum);
  const viewport = page.getViewport({ scale: 1.4 });
  const canvas = document.getElementById('pdfCanvas');
  const ctx = canvas.getContext('2d');
  canvas.width = viewport.width;
  canvas.height = viewport.height;

  const task = page.render({ canvasContext: ctx, viewport });
  currentRenderTask = task;

  try {
    await task.promise;
  } catch (e) {
    // RenderingCancelledException — нормальное поведение, игнорируем
    if (e && e.name !== 'RenderingCancelledException') {
      console.warn('[render]', e);
    }
    return;
  } finally {
    currentRenderTask = null;
  }

  document.getElementById('slideCounter').textContent =
    pageNum + ' / ' + pdfDoc.numPages;
  renderQuestions();
}

// ─── Вопросы на слайде ───
function renderQuestions() {
  const box = document.getElementById('questionList');
  if (!box) return;
  box.innerHTML = '';
  const onSlide = questions.filter(q => q.slide === pageNum);
  if (!onSlide.length) {
    box.innerHTML = '<p class="hint">Нет вопросов на этом слайде</p>';
    return;
  }
  onSlide.forEach(q => {
    const div = document.createElement('div');
    div.className = 'list';
    div.innerHTML = `<li><strong>${q.text}</strong>
      <small>Вариантов: ${q.options.length}</small></li>`;
    box.appendChild(div);
  });
}

// ─── Навигация по слайдам ───
document.getElementById('prevSlide').onclick = () => {
  if (pageNum > 1) { pageNum--; renderPage(); }
};
document.getElementById('nextSlide').onclick = () => {
  if (pdfDoc && pageNum < pdfDoc.numPages) { pageNum++; renderPage(); }
};

// ─── QR-код ───
document.getElementById('qrBtn').onclick = () => {
  if (!currentLecture) {
    alert('Сначала выберите лекцию');
    return;
  }
  const holder = document.getElementById('qrHolder');
  holder.innerHTML = '';
  const url = location.origin + '/student?lecture=' + currentLecture.id;
  new QRCode(holder, { text: url, width: 240, height: 240 });
  document.getElementById('qrLink').textContent = url;
  document.getElementById('qrModal').classList.remove('hidden');
};

// ─── Квиз: запуск / остановка ───
document.getElementById('startQuizBtn').onclick = async () => {
  if (!currentLecture) {
    alert('Сначала выберите лекцию');
    return;
  }
  try {
    await api('/api/quiz/' + currentLecture.id + '/start', { method: 'POST' });
    quizActive = true;
    updateQuizUI();
    startStatsPolling();
  } catch (e) {
    alert('Не удалось запустить квиз: ' + e.message);
  }
};

document.getElementById('stopQuizBtn').onclick = async () => {
  if (!currentLecture) return;
  try {
    await api('/api/quiz/' + currentLecture.id + '/stop', { method: 'POST' });
    quizActive = false;
    updateQuizUI();
    stopStatsPolling();
  } catch (e) {
    alert('Не удалось остановить квиз: ' + e.message);
  }
};

// ─── Квиз: UI ───
function updateQuizUI() {
  const status = document.getElementById('quizStatus');
  const startBtn = document.getElementById('startQuizBtn');
  const stopBtn = document.getElementById('stopQuizBtn');
  if (!status || !startBtn || !stopBtn) return;

  if (quizActive) {
    status.textContent = '● Квиз идёт';
    status.classList.add('active');
    startBtn.classList.add('hidden');
    stopBtn.classList.remove('hidden');
  } else {
    status.textContent = 'Квиз не запущен';
    status.classList.remove('active');
    startBtn.classList.remove('hidden');
    stopBtn.classList.add('hidden');
  }
}

function syncQuizUI() {
  quizActive = false;
  updateQuizUI();
  stopStatsPolling();
  const box = document.getElementById('liveStats');
  if (box) box.innerHTML = '<p class="hint">Запустите квиз, чтобы видеть ответы</p>';
}

// ─── Квиз: live-статистика ───
function startStatsPolling() {
  stopStatsPolling();
  refreshStats();
  statsInterval = setInterval(refreshStats, 2000);
}

function stopStatsPolling() {
  if (statsInterval) {
    clearInterval(statsInterval);
    statsInterval = null;
  }
}

async function refreshStats() {
  if (!currentLecture) return;
  const box = document.getElementById('liveStats');
  if (!box) return;
  try {
    const lb = await api('/api/quiz/' + currentLecture.id + '/leaderboard');
    const list = lb || [];

    if (!list.length) {
      box.innerHTML = '<p class="hint">Пока никто не ответил</p>';
      return;
    }

    const html = [
      `<div class="stat-row"><span>Ответили</span><span class="stat-value">${list.length}</span></div>`,
      `<div class="stat-row"><span>Лидер</span><span class="stat-value">${list[0].name}</span></div>`,
      '<h4 style="font-size:13px;color:var(--muted);margin-top:8px;text-transform:uppercase;letter-spacing:1px;">Топ-5</h4>',
      '<ol class="leaderboard-mini">',
    ];
    list.slice(0, 5).forEach(e => {
      html.push(`<li><span class="name">${e.name}</span><span class="score">${e.score}</span></li>`);
    });
    html.push('</ol>');
    box.innerHTML = html.join('');
  } catch (e) {
    console.warn('[stats]', e.message);
  }
}

// ─── Загрузка PDF ───
document.getElementById('pdfInput').addEventListener('change', async (e) => {
  const file = e.target.files[0];
  if (!file) return;

  const title = prompt('Название лекции:', file.name);
  if (!title) {
    e.target.value = '';
    return;
  }

  const fd = new FormData();
  fd.append('pdf', file);
  fd.append('title', title);
  fd.append('owner_id', me.id);

  try {
    await api('/api/lectures', { method: 'POST', body: fd });
    await loadLectures();
  } catch (err) {
    alert('Ошибка загрузки: ' + err.message);
  }

  e.target.value = '';
});

// ─── Точка входа ───
bootstrap();
