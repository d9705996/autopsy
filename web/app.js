async function login() {
  const username = document.getElementById('u').value;
  const password = document.getElementById('p').value;
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({username, password})
  });
  if (!res.ok) return alert('login failed');
  document.getElementById('login').classList.add('hidden');
  document.getElementById('dashboard').classList.remove('hidden');
  await loadData();
}

async function seedCriticalAlert() {
  await fetch('/api/alerts', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      source: 'grafana',
      title: 'API 5xx spike',
      description: 'Timeout errors above SLO objective',
      severity: 'critical',
      labels: {service: 'api', metric: 'http_5xx_rate'}
    })
  });
  await loadData();
}

async function loadData() {
  const [alerts, incidents, postmortems, playbooks, oncall] = await Promise.all([
    fetch('/api/alerts').then(r => r.json()),
    fetch('/api/incidents').then(r => r.json()),
    fetch('/api/postmortems').then(r => r.json()),
    fetch('/api/playbooks').then(r => r.json()),
    fetch('/api/oncall').then(r => r.json())
  ]);
  document.getElementById('out').textContent = JSON.stringify({alerts, incidents, postmortems, playbooks, oncall}, null, 2);
}
