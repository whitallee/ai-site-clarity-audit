const form = document.getElementById('audit-form');
const urlInput = document.getElementById('url-input');
const submitBtn = document.getElementById('submit-btn');
const loading = document.getElementById('loading');
const errorBox = document.getElementById('error-box');
const results = document.getElementById('results');

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  const url = urlInput.value.trim();
  if (!url) return;
  await runAudit(url);
});

async function runAudit(url) {
  setLoading(true);
  hideError();
  results.classList.add('hidden');

  try {
    const res = await fetch('/api/audit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });

    const text = await res.text();
    if (!res.ok) throw new Error(text || `Server error: ${res.status}`);

    const data = JSON.parse(text);
    renderResults(data.id, data.audit);
  } catch (err) {
    showError(err.message);
  } finally {
    setLoading(false);
  }
}

function renderResults(id, audit) {
  const scoreEl = document.getElementById('clarity-score');
  scoreEl.textContent = audit.clarity_score;
  scoreEl.className = 'score-number ' + scoreClass(audit.clarity_score);

  document.getElementById('what-it-does').textContent = audit.what_it_does;
  document.getElementById('pdf-link').href = `/api/audit/${id}/pdf`;

  const suggestionsEl = document.getElementById('suggestions');
  suggestionsEl.innerHTML = (audit.suggestions || []).map(s => `<li>${s}</li>`).join('');

  results.classList.remove('hidden');
  results.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function scoreClass(score) {
  if (score >= 8) return 'good';
  if (score >= 5) return 'warn';
  return 'bad';
}

function setLoading(on) {
  loading.classList.toggle('hidden', !on);
  submitBtn.disabled = on;
  submitBtn.textContent = on ? 'Analyzing…' : 'Analyze';
}

function showError(msg) {
  errorBox.textContent = `Error: ${msg}`;
  errorBox.classList.remove('hidden');
}

function hideError() {
  errorBox.classList.add('hidden');
}
