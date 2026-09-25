pdfjsLib.GlobalWorkerOptions.workerSrc =
  "https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js";

let currentLecture = null;
let pdfDoc = null;
let pageNum = 1;
let questions = [];

async function api(url, opts = {}) {
  const r = await fetch(url, {credentials:'include', ...opts});
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

// Загрузка списка лекций
async function loadLectures() {
  const me = await api('/api/login/me').catch(() => null);
  // для простоты берём owner_id из localStorage
  const uid = localStorage.getItem('uid') || '';
  const items = await api('/api/lectures?owner_id=' + uid);
  const ul = document.getElementById('lectureList');
  ul.innerHTML = '';
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
  renderPage();
}

async function renderPage() {
  const page = await pdfDoc.getPage(pageNum);
  const viewport = page.getViewport({scale: 1.4});
  const canvas = document.getElementById('pdfCanvas');
  const ctx = canvas.getContext('2d');
  canvas.width = viewport.width;
  canvas.height = viewport.height;
  await page.render({canvasContext: ctx, viewport}).promise;
  document.getElementById('slideCounter').textContent =
    pageNum + ' / ' + pdfDoc.numPages;
  renderQuestions();
}

function renderQuestions() {
  const box = document.getElementById('questionList');
  box.innerHTML = '';
  questions.filter(q => q.slide === pageNum).forEach(q => {
    const div = document.createElement('div');
    div.className = 'list';
    div.innerHTML = `<li><strong>${q.text}</strong>
      <small>Вариантов: ${q.options.length}</small></li>`;
    box.appendChild(div);
  });
  if (!box.children.length) box.innerHTML = '<p class="hint">Нет вопросов на этом слайде</p>';
}

// Навигация
document.getElementById('prevSlide').onclick = () => {
  if (pageNum > 1) { pageNum--; renderPage(); }
};
document.getElementById('nextSlide').onclick = () => {
  if (pageNum < pdfDoc.numPages) { pageNum++; renderPage(); }
};

// QR
document.getElementById('qrBtn').onclick = () => {
  const holder = document.getElementById('qrHolder');
  holder.innerHTML = '';
  const url = location.origin + '/student?lecture=' + currentLecture.id;
  new QRCode(holder, {text: url, width: 240, height: 240});
  document.getElementById('qrLink').textContent = url;
  document.getElementById('qrModal').classList.remove('hidden');
};

// Новая лекция
document.getElementById('newLecture').onclick = () => {
  const input = document.createElement('input');
  input.type = 'file';
  input.accept = 'application/pdf';
  input.onchange = async () => {
    const file = input.files[0];
    if (!file) return;
    const title = prompt('Название лекции:', file.name);
    if (!title) return;
    const fd = new FormData();
    fd.append('pdf', file);
    fd.append('title', title);
    fd.append('owner_id', localStorage.getItem('uid') || '');
    await api('/api/lectures', {method:'POST', body: fd});
    loadLectures();
  };
  input.click();
};

loadLectures();
