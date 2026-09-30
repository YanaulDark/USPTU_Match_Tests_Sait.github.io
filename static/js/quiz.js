const params = new URLSearchParams(location.search);
const lectureId = params.get('lecture');

let currentQuestion = null;
let progress = null;
let chosenIndex = null;
let total = 0;

async function api(url, opts = {}) {
  const r = await fetch(url, { credentials: 'include', ...opts });
  if (!r.ok) {
    const text = await r.text();
    throw new Error(`${r.status}: ${text}`);
  }
  return r.json();
}

async function loadProgress() {
  if (!lectureId) {
    document.body.innerHTML = '<div style="padding:40px;color:#ef4444;font:16px sans-serif">Нет lecture в URL. Откройте ссылку из QR.</div>';
    return;
  }
  const data = await api('/api/quiz/' + lectureId + '/progress');

  if (data.finished) {
    return showFinish(data);
  }

  currentQuestion = data.question;
  progress = data.progress;
  total = data.total;

  if (!currentQuestion) {
    return showFinish(data);
  }

  document.getElementById('quizStep').textContent =
    'Вопрос ' + (progress.index + 1) + ' / ' + total;
  document.getElementById('quizScore').textContent =
    progress.score + ' ' + plural(progress.score, 'балл', 'балла', 'баллов');

  renderQuestion(currentQuestion);
}

function renderQuestion(q) {
  showScreen('screenQuestion');
  chosenIndex = null;

  document.getElementById('questionText').textContent = q.text;

  const box = document.getElementById('questionOptions');
  box.innerHTML = '';

  q.options.forEach((opt, i) => {
    const b = document.createElement('button');
    b.className = 'option';
    b.textContent = opt;
    b.onclick = () => {
      box.querySelectorAll('.option').forEach(x => x.classList.remove('selected'));
      b.classList.add('selected');
      chosenIndex = i;
      setTimeout(submitAnswer, 200);
    };
    box.appendChild(b);
  });
}

async function submitAnswer() {
  if (chosenIndex === null || !currentQuestion) return;

  try {
    const res = await api('/api/quiz/' + lectureId + '/answer', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        question_id: currentQuestion.id,
        choice: chosenIndex,
      }),
    });

    progress = res.progress;
    document.getElementById('quizScore').textContent =
      progress.score + ' ' + plural(progress.score, 'балл', 'балла', 'баллов');

    showResult(res);
  } catch (e) {
    alert('Не удалось отправить ответ: ' + e.message);
  }
}

function showResult(res) {
  showScreen('screenResult');

  const verdict = document.getElementById('verdict');
  verdict.textContent = res.correct ? 'Правильно!' : 'Неправильно';
  verdict.className = 'quiz-verdict ' + (res.correct ? 'correct' : 'wrong');

  document.getElementById('correctAnswer').textContent =
    'Правильный ответ: ' + currentQuestion.options[res.correct_index];

  renderChart(currentQuestion.options, res.stats, res.correct_index);

  const nextBtn = document.getElementById('nextBtn');
  nextBtn.textContent = progress.finished ? 'К результатам →' : 'Дальше →';
  nextBtn.onclick = () => {
    if (progress.finished) return showFinish({ progress });
    loadProgress();
  };
}

function renderChart(options, stats, correctIndex) {
  const chart = document.getElementById('chart');
  chart.innerHTML = '';

  const max = Math.max(1, ...Object.values(stats));
  const labels = ['A', 'B', 'C', 'D', 'E', 'F'];

  options.forEach((opt, i) => {
    const count = stats[i] || 0;
    const pct = (count / max) * 100;

    const row = document.createElement('div');
    row.className = 'chart-row';
    row.innerHTML = `
      <div class="chart-label">${labels[i] || i + 1}</div>
      <div class="chart-bar-wrap">
        <div class="chart-bar ${i === correctIndex ? 'correct' : ''}"
             style="width: ${pct}%"></div>
      </div>
      <div class="chart-count">${count}</div>`;
    chart.appendChild(row);
  });
}

async function showFinish(data) {
  showScreen('screenFinish');

  const p = data.progress || progress;
  document.getElementById('finalScore').textContent =
    'Ваш результат: ' + p.score + ' из ' + p.total + ' ' +
    plural(p.score, 'балл', 'балла', 'баллов');

  const lb = await api('/api/quiz/' + lectureId + '/leaderboard');
  const list = document.getElementById('leaderboard');
  list.innerHTML = '';
  (lb || []).forEach(entry => {
    const li = document.createElement('li');
    li.innerHTML = `<span class="lb-name">${entry.name}</span>
      <span class="lb-score">${entry.score}</span>`;
    list.appendChild(li);
  });
}

function showScreen(id) {
  ['screenQuestion', 'screenResult', 'screenFinish'].forEach(x => {
    const el = document.getElementById(x);
    if (el) el.classList.toggle('hidden', x !== id);
  });
}

function plural(n, one, few, many) {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
}

loadProgress().catch(e => {
  console.error('quiz error:', e);
  document.body.innerHTML = `
    <div style="padding:40px;color:#ef4444;font:16px sans-serif">
      <h2>Ошибка</h2>
      <p>${e.message}</p>
      <a href="/" style="color:#3b82f6">На главную</a>
    </div>`;
});
