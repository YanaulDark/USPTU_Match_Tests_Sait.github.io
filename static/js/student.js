const params = new URLSearchParams(location.search);
const lectureId = params.get('lecture');

async function api(url, opts = {}) {
  const r = await fetch(url, {credentials:'include', ...opts});
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

async function load() {
  const data = await api('/api/lectures/' + lectureId);
  document.getElementById('lectureName').textContent = data.lecture.title;
  renderQuestions(data.questions || []);
}

function renderQuestions(qs) {
  const box = document.getElementById('questions');
  box.innerHTML = '';
  qs.forEach(q => {
    const card = document.createElement('div');
    card.className = 'question-card';
    card.innerHTML = `<h3>${q.text}</h3>`;
    q.options.forEach((opt, i) => {
      const b = document.createElement('button');
      b.className = 'option';
      b.textContent = opt;
      b.onclick = async () => {
        const res = await api('/api/answers?lecture_id=' + lectureId, {
          method: 'POST',
          headers: {'Content-Type':'application/json'},
          body: JSON.stringify({question_id: q.id, student_id:'me', choice: i})
        });
        card.querySelectorAll('.option').forEach((o, j) => {
          o.disabled = true;
          if (j === q.correct) o.classList.add('correct');
          else if (j === i) o.classList.add('wrong');
        });
      };
      card.appendChild(b);
    });
    box.appendChild(card);
  });
}

load();
