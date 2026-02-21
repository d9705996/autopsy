let alertsCache = [];
let selectedAlertID = '';

async function request(path, options = {}) {
  const res = await fetch(path, options);
  if (!res.ok) {
    throw new Error(await res.text());
  }
  return res.json();
}

async function login() {
  const username = document.getElementById('u').value;
  const password = document.getElementById('p').value;
  await request('/api/login', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({username, password})
  });
  document.getElementById('login').classList.add('hidden');
  document.getElementById('dashboard').classList.remove('hidden');
  await loadSession();
  await loadData();
}

async function logout() {
  await request('/api/logout', { method: 'POST' });
  document.getElementById('dashboard').classList.add('hidden');
  document.getElementById('login').classList.remove('hidden');
}

async function loadSession() {
  try {
    const me = await request('/api/me');
    document.getElementById('whoami').textContent = `Logged in as ${me.username} (${me.roles.join(', ')})`;
    const isAdmin = me.roles.includes('admin');
    document.getElementById('admin-panel').classList.toggle('hidden', !isAdmin);
    if (isAdmin) {
      await Promise.all([loadUsers(), loadRoles(), loadInvites()]);
    }
  } catch (_) {}
}

async function seedCriticalAlert() {
  await request('/api/alerts', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      source: 'grafana',
      title: 'API 5xx spike',
      description: 'Timeout errors above SLO objective and customer checkout impact',
      severity: 'critical',
      labels: {service: 'api', metric: 'http_5xx_rate'}
    })
  });
  await loadData();
}

async function seedAutoFixAlert() {
  await request('/api/alerts', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      source: 'queue-monitor',
      title: 'Retry queue growth',
      description: 'retry backlog increased by 25% over baseline',
      severity: 'warning',
      labels: {service: 'worker', metric: 'retry_queue_depth'}
    })
  });
  await loadData();
}

function badgeClassForDecision(decision) {
  if (decision === 'start_incident') return 'border-rose-400/40 bg-rose-500/10 text-rose-300';
  if (decision === 'auto_fix') return 'border-emerald-400/40 bg-emerald-500/10 text-emerald-300';
  if (decision === 'create_issue') return 'border-amber-400/40 bg-amber-500/10 text-amber-300';
  return 'border-slate-600 bg-slate-800 text-slate-300';
}

function renderAlerts(alerts) {
  const host = document.getElementById('alerts-list');
  if (!alerts.length) {
    host.innerHTML = '<p class="text-sm text-slate-400">No alerts received yet.</p>';
    renderTimeline(null);
    return;
  }
  if (!selectedAlertID || !alerts.some((a) => a.id === selectedAlertID)) {
    selectedAlertID = alerts[0].id;
  }

  host.innerHTML = alerts.map((alert) => {
    const triage = alert.triage || {};
    const decision = triage.decision || 'pending';
    const active = alert.id === selectedAlertID;
    return `
      <article class="cursor-pointer rounded-xl border p-4 transition ${active ? 'border-cyan-400/60 bg-cyan-500/5' : 'border-slate-800 bg-slate-950 hover:border-slate-700'}" onclick="selectAlert('${alert.id}')">
        <div class="mb-2 flex items-center justify-between gap-3">
          <h4 class="font-medium text-slate-100">${alert.title}</h4>
          <span class="text-xs text-slate-500">${alert.id}</span>
        </div>
        <div class="mb-2 flex flex-wrap gap-2 text-xs">
          <span class="rounded-full border border-slate-700 bg-slate-800 px-2 py-1 text-slate-200">${alert.severity}</span>
          <span class="rounded-full border border-blue-400/40 bg-blue-500/10 px-2 py-1 text-blue-300">${alert.status || 'received'}</span>
          <span class="rounded-full border px-2 py-1 ${badgeClassForDecision(decision)}">${decision}</span>
        </div>
        <p class="text-sm text-slate-400">${alert.description}</p>
      </article>
    `;
  }).join('');

  renderTimeline(alerts.find((a) => a.id === selectedAlertID) || null);
}

function renderTimeline(alert) {
  const timelineEmpty = document.getElementById('timeline-empty');
  const timelineList = document.getElementById('timeline-list');
  const outcome = document.getElementById('triage-outcome');

  if (!alert || !alert.triage || !alert.triage.timeline) {
    timelineEmpty.classList.remove('hidden');
    timelineList.classList.add('hidden');
    timelineList.innerHTML = '';
    outcome.classList.add('hidden');
    outcome.innerHTML = '';
    return;
  }

  timelineEmpty.classList.add('hidden');
  timelineList.classList.remove('hidden');
  timelineList.innerHTML = alert.triage.timeline.map((step) => `
    <li class="relative">
      <span class="absolute -left-[22px] top-1.5 h-2.5 w-2.5 rounded-full bg-cyan-400"></span>
      <p class="text-sm font-medium text-slate-100">${step.phase} <span class="text-xs text-slate-500">· ${new Date(step.timestamp).toLocaleTimeString()}</span></p>
      <p class="text-sm text-slate-400">${step.detail}</p>
    </li>
  `).join('');

  const triage = alert.triage;
  const fixPlan = triage.autoFixPlan && triage.autoFixPlan.length
    ? `<h4 class="mt-3 text-sm font-semibold text-slate-200">Auto-fix plan</h4><ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-slate-300">${triage.autoFixPlan.map((p) => `<li>${p}</li>`).join('')}</ul>`
    : '';
  const issue = triage.issueTitle ? `<p class="mt-2 text-sm text-slate-300"><strong>Issue to create:</strong> ${triage.issueTitle}</p>` : '';
  outcome.classList.remove('hidden');
  outcome.innerHTML = `
    <h4 class="text-sm font-semibold text-cyan-300">Decision: ${triage.decision}</h4>
    <p class="mt-2 text-sm text-slate-300">${triage.summary}</p>
    <p class="mt-2 text-sm text-slate-300"><strong>Likely root cause:</strong> ${triage.likelyRootCause}</p>
    ${issue}
    ${fixPlan}
  `;
}

function selectAlert(id) {
  selectedAlertID = id;
  renderAlerts(alertsCache);
}

async function loadData() {
  const [alerts, incidents, postmortems, playbooks, oncall] = await Promise.all([
    request('/api/alerts'),
    request('/api/incidents'),
    request('/api/postmortems'),
    request('/api/playbooks'),
    request('/api/oncall')
  ]);
  alertsCache = alerts;
  renderAlerts(alerts);
  document.getElementById('out').textContent = JSON.stringify({alerts, incidents, postmortems, playbooks, oncall}, null, 2);
}

async function loadUsers() {
  const users = await request('/api/admin/users');
  document.getElementById('users-out').textContent = JSON.stringify(users, null, 2);
}

async function createUser() {
  const username = document.getElementById('new-username').value;
  const displayName = document.getElementById('new-display').value;
  const password = document.getElementById('new-password').value;
  const roles = document.getElementById('new-roles').value.split(',').map((s) => s.trim()).filter(Boolean);
  await request('/api/admin/users', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({username, displayName, password, roles})
  });
  await loadUsers();
}

async function loadRoles() {
  const roles = await request('/api/admin/roles');
  document.getElementById('roles-out').textContent = JSON.stringify(roles, null, 2);
}

async function createRole() {
  const name = document.getElementById('role-name').value;
  const description = document.getElementById('role-desc').value;
  const permissions = document.getElementById('role-perms').value.split(',').map((s) => s.trim()).filter(Boolean);
  await request('/api/admin/roles', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({name, description, permissions})
  });
  await loadRoles();
}

async function loadInvites() {
  const invites = await request('/api/admin/invites');
  document.getElementById('invites-out').textContent = JSON.stringify(invites, null, 2);
}

async function createInvite() {
  const email = document.getElementById('invite-email').value;
  const role = document.getElementById('invite-role').value;
  await request('/api/admin/invites', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({email, role})
  });
  await loadInvites();
}
