pdfjsLib.GlobalWorkerOptions.workerSrc =
  "https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js";

let me = null;
let currentLecture = null;
let pdfDoc = null;
let pageNum = 1;
let questions = [];

async function loadLectures() {
  const items = await api('/api/lectures?owner_id=' + encodeURIComponent(me.id));
  const ul = document.getElementById('lectureList');
  if (!ul) return;
  ul.innerHTML = '';

  const list = items || [];        // ← защита от null
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

async function api(url, opts = {}) {
  const r = await fetch(url, { credentials: 'include', ...opts });
  if (!r.ok) {
    const text = await r.text();
    throw new Error(`${r.status}: ${text}`);
  }
  return r.json();
}

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

async function loadLectures() {
  const items = await api('/api/lectures?owner_id=' + encodeURIComponent(me.id));
  const ul = document.getElementById('lectureList');
  if (!ul) return;
  ul.innerHTML = '';
  if (!items.length) {
    ul.innerHTML = '<li style="opacity:.5;cursor:default">Пока нет лекций</li>';
    return;
  }
  items.forEach(l => {
    const li = document.createElement('li');
    li.textContent = l.title || l.id;
    li.onclick = () => selectLecture(l);
    ul.appendChild(li);
  });
}

async function selectLecture(l) {
  currentLecture = l;
  document.getElementById('lectureTitle').textContent = l.title;
  const data = await api('/api/lectures/' + l.id);
  questions = data.questions || [];
  renderQuestions();
  await loadPDF(l.pdf_path);
}

async function loadPDF(url) {
  const task = pdfjsLib.getDocument(url);
  pdfDoc = await task.promise;
  pageNum = 1;
  await renderPage();
}

async function renderPage() {
  const page = await pdfDoc.getPage(pageNum);
  const viewport = page.getViewport({ scale: 1.4 });
  const canvas = document.getElementById('pdfCanvas');
  const ctx = canvas.getContext('2d');
  canvas.width = viewport.width;
  canvas.height = viewport.height;
  await page.render({ canvasContext: ctx, viewport }).promise;
  document.getElementById('slideCounter').textContent = pageNum + ' / ' + pdfDoc.numPages;
  renderQuestions();
}

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

document.getElementById('prevSlide').onclick = () => {
  if (pageNum > 1) { pageNum--; renderPage(); }
};
document.getElementById('nextSlide').onclick = () => {
  if (pdfDoc && pageNum < pdfDoc.numPages) { pageNum++; renderPage(); }
};

document.getElementById('qrBtn').onclick = () => {
  if (!currentLecture) return;
  const holder = document.getElementById('qrHolder');
  holder.innerHTML = '';
  const url = location.origin + '/student?lecture=' + currentLecture.id;
  new QRCode(holder, { text: url, width: 240, height: 240 });
  document.getElementById('qrLink').textContent = url;
  document.getElementById('qrModal').classList.remove('hidden');
};

// Открытие диалога по клику на label
const pdfLabel = document.querySelector('label[for="pdfInput"]');
if (pdfLabel) {
  pdfLabel.addEventListener('click', (e) => {
    e.preventDefault();
    document.getElementById('pdfInput').click();
  });
}

// Обработка выбранного файла
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

// ─── точка входа ───
bootstrap();
