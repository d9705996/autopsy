let alertsCache = [];
let selectedAlertID = '';
let toolsCache = [];
let editingToolID = '';

// Incremented every time bootPage() is called.  Each invocation captures the
// value at its start; only the call whose generation still matches the current
// value is allowed to change the login/app-shell visibility.  This prevents
// a stale in-flight /api/me (401) from hiding the app shell AFTER a
// successful login has already shown it.
let _bootGen = 0;

// Sentinel error used to distinguish HTTP 401 from other request failures.
class UnauthorizedError extends Error {
  constructor() { super('unauthorized'); }
}

async function request(path, options = {}) {
  const res = await fetch(path, {
    credentials: 'include',
    cache: 'no-store',
    ...options,
  });
  if (res.status === 401) {
    // Throw a typed error — callers decide whether to show the login form.
    // Do NOT call showLoggedOut() here: a stale 401 from an earlier page load
    // must not overwrite the UI after a successful login.
    throw new UnauthorizedError();
  }
  if (!res.ok) {
    throw new Error(await res.text());
  }
  return res.json();
}

function showLoggedOut() {
  document.getElementById('login')?.classList.remove('hidden');
  document.getElementById('app-shell')?.classList.add('hidden');
}

function showLoggedIn() {
  document.getElementById('login')?.classList.add('hidden');
  document.getElementById('app-shell')?.classList.remove('hidden');
  showLoginError('');
}

function showLoginError(msg) {
  let el = document.getElementById('login-error');
  if (!el) return;
  el.textContent = msg;
  el.classList.toggle('hidden', !msg);
}

async function login() {
  showLoginError('');
  const username = document.getElementById('u').value;
  const password = document.getElementById('p').value;
  if (!username || !password) {
    showLoginError('Username and password are required.');
    return;
  }
  try {
    // Use fetch directly so the 401-interceptor in request() does not
    // interfere with the login endpoint — a 401 here means bad credentials,
    // not an expired session.
    const res = await fetch('/api/login', {
      method: 'POST',
      credentials: 'include',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    if (res.status === 401) {
      showLoginError('Invalid username or password.');
      return;
    }
    if (!res.ok) {
      showLoginError(`Login failed: ${await res.text()}`);
      return;
    }
    await bootPage();
  } catch (err) {
    showLoginError('Unable to reach the server. Please try again.');
  }
}

async function logout() {
  // Use fetch directly — logout is a public endpoint and should always succeed
  // regardless of session state; bypassing the 401-interceptor avoids showing
  // the login screen twice.
  await fetch('/api/logout', { method: 'POST', credentials: 'include', cache: 'no-store' }).catch(() => { });
  showLoggedOut();
}

async function bootPage() {
  // Grab this boot cycle's generation number before the first await so that
  // any earlier, still-in-flight bootPage() cannot clobber our UI state.
  const gen = ++_bootGen;

  // Phase 1: verify the session.
  let me;
  try {
    me = await request('/api/me');
  } catch (_) {
    // Only show the login form if we are still the latest boot cycle.
    if (gen === _bootGen) showLoggedOut();
    return;
  }

  // Abort if a newer bootPage() call has already taken over.
  if (gen !== _bootGen) return;

  // Phase 2: session confirmed — reveal the app shell.
  showLoggedIn();
  const who = document.getElementById('whoami');
  if (who) {
    who.textContent = `Logged in as ${me.username} (${me.roles.join(', ')})`;
  }
  const isAdmin = me.roles.includes('admin');
  document.querySelectorAll('[data-admin-only="true"]').forEach((el) => {
    el.classList.toggle('hidden', !isAdmin);
  });

  const page = document.body.dataset.page;
  try {
    if (page === 'dashboard') await loadDashboard();
    if (page === 'alerts') await loadAlertsPage();
    if (page === 'incidents') await loadIncidentsPage();
    if (page === 'tools') await loadToolsPage();
    if (page === 'admin' && isAdmin) await Promise.all([loadUsers(), loadRoles(), loadInvites()]);
  } catch (err) {
    if (err instanceof UnauthorizedError && gen === _bootGen) {
      // Session expired mid-page-load; show login form.
      showLoggedOut();
    } else {
      console.error('Page load error:', err);
    }
  }
}

function navMarkup() {
  return `
  <header class="mb-6 rounded-2xl border border-slate-800 bg-slate-900/70 p-6 shadow-xl shadow-slate-950/40">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div class="flex items-center gap-4">
        <div class="rounded-xl bg-gradient-to-br from-cyan-500 to-blue-600 p-3">
          <svg class="h-8 w-8 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M4 19h16M6 17l2-9h8l2 9M9 8a3 3 0 0 1 6 0" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </div>
        <div>
          <h1 class="text-2xl font-semibold tracking-tight">Autopsy Incident Response</h1>
          <p class="text-sm text-slate-400">Enterprise alert triage and MCP tool management.</p>
        </div>
      </div>
      <button onclick="logout()" class="rounded-lg border border-slate-700 px-3 py-2 text-sm hover:bg-slate-800">Logout</button>
    </div>
    <nav class="mt-4 flex flex-wrap gap-2 text-sm">
      <a href="/" class="rounded-lg border border-slate-700 px-3 py-1.5 hover:bg-slate-800">Dashboard</a>
      <a href="/alerts.html" class="rounded-lg border border-slate-700 px-3 py-1.5 hover:bg-slate-800">Alerts</a>
      <a href="/incidents.html" class="rounded-lg border border-slate-700 px-3 py-1.5 hover:bg-slate-800">Incidents</a>
      <a href="/tools.html" class="rounded-lg border border-slate-700 px-3 py-1.5 hover:bg-slate-800">MCP Tools</a>
      <a href="/admin.html" data-admin-only="true" class="rounded-lg border border-slate-700 px-3 py-1.5 hover:bg-slate-800">Admin</a>
    </nav>
    <p id="whoami" class="mt-3 text-xs text-slate-400"></p>
  </header>`;
}

async function seedCriticalAlert() {
  await request('/api/alerts', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ source: 'grafana', title: 'API 5xx spike', description: 'Timeout errors above SLO objective and customer checkout impact', severity: 'critical', labels: { service: 'api', metric: 'http_5xx_rate' } })
  });
  if (document.body.dataset.page === 'dashboard') await loadDashboard();
  if (document.body.dataset.page === 'alerts') await loadAlertsPage();
}

async function seedAutoFixAlert() {
  await request('/api/alerts', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ source: 'queue-monitor', title: 'Retry queue growth', description: 'retry backlog increased by 25% over baseline', severity: 'warning', labels: { service: 'worker', metric: 'retry_queue_depth' } })
  });
  if (document.body.dataset.page === 'dashboard') await loadDashboard();
  if (document.body.dataset.page === 'alerts') await loadAlertsPage();
}


async function loadPublicStatusPage() {
  const data = await request('/api/statuspage');
  document.getElementById('overall-status').textContent = data.overallStatus.replaceAll('_', ' ');
  document.getElementById('status-updated').textContent = new Date(data.updatedAt).toLocaleString();
  const servicesHost = document.getElementById('service-availability');
  servicesHost.innerHTML = data.services.map((svc) => `
    <article class="rounded-xl border border-slate-800 bg-slate-900 p-4">
      <div class="flex items-center justify-between gap-3">
        <h3 class="text-base font-semibold">${svc.service}</h3>
        <span class="text-sm text-cyan-300">${svc.availabilityPercent.toFixed(2)}%</span>
      </div>
      <p class="mt-2 text-xs text-slate-500">Downtime: ${svc.downtimeMinutes} minutes · Period: ${new Date(svc.periodStart).toLocaleString()} - ${new Date(svc.periodEnd).toLocaleString()}</p>
    </article>
  `).join('') || '<p class="text-slate-400">No service-impacting incidents in this period.</p>';

  const host = document.getElementById('public-incidents');
  host.innerHTML = data.incidents.map((inc) => `
    <article class="rounded-xl border border-slate-800 bg-slate-900 p-4">
      <div class="flex items-center justify-between gap-3">
        <h3 class="text-lg font-semibold">${inc.title}</h3>
        <span class="rounded-full border border-rose-500/40 bg-rose-500/10 px-2 py-1 text-xs text-rose-300">${inc.status}</span>
      </div>
      <p class="mt-2 text-sm text-slate-300">${inc.currentMessage}</p>
      <p class="mt-2 text-xs text-slate-500">Service: ${inc.service} · Declared: ${new Date(inc.declaredAt).toLocaleString()} · Severity: ${inc.severity}</p>
      <a class="mt-2 inline-block text-sm text-cyan-300 underline" href="${inc.statusPageUrl}">Status page link</a>
      <ul class="mt-3 list-disc space-y-1 pl-5 text-sm text-slate-300">${(inc.responsePlaybook || []).map((step) => `<li>${step}</li>`).join('')}</ul>
    </article>
  `).join('') || '<p class="text-slate-400">All systems operating normally.</p>';
}

async function loadDashboard() {
  const [alerts, incidents, tools] = await Promise.all([request('/api/alerts'), request('/api/incidents'), request('/api/tools')]);
  document.getElementById('kpi-alerts').textContent = alerts.length;
  document.getElementById('kpi-incidents').textContent = incidents.length;
  document.getElementById('kpi-tools').textContent = tools.length;
  document.getElementById('recent-alerts').innerHTML = alerts.slice(0, 5).map((a) => `<li>${a.title} · <span class="text-slate-400">${a.status}</span></li>`).join('') || '<li class="text-slate-500">No alerts yet.</li>';
  document.getElementById('recent-incidents').innerHTML = incidents.slice(0, 5).map((i) => `<li>${i.title} · <span class="text-slate-400">${i.status}</span></li>`).join('') || '<li class="text-slate-500">No incidents yet.</li>';
}

function badgeClassForDecision(decision) {
  if (decision === 'start_incident') return 'border-rose-400/40 bg-rose-500/10 text-rose-300';
  if (decision === 'auto_fix') return 'border-emerald-400/40 bg-emerald-500/10 text-emerald-300';
  if (decision === 'create_issue') return 'border-amber-400/40 bg-amber-500/10 text-amber-300';
  return 'border-slate-600 bg-slate-800 text-slate-300';
}

async function loadAlertsPage() {
  alertsCache = await request('/api/alerts');
  const host = document.getElementById('alerts-list');
  if (!alertsCache.length) {
    host.innerHTML = '<p class="text-sm text-slate-400">No alerts received yet.</p>';
    renderTimeline(null);
    return;
  }
  if (!selectedAlertID || !alertsCache.some((a) => a.id === selectedAlertID)) selectedAlertID = alertsCache[0].id;

  host.innerHTML = alertsCache.map((alert) => {
    const triage = alert.triage || {};
    const decision = triage.decision || 'pending';
    const active = alert.id === selectedAlertID;
    return `<article class="cursor-pointer rounded-xl border p-4 ${active ? 'border-cyan-400/60 bg-cyan-500/5' : 'border-slate-800 bg-slate-950'}" onclick="selectAlert('${alert.id}')">
      <div class="mb-2 flex items-center justify-between"><h4>${alert.title}</h4><span class="text-xs text-slate-500">${alert.id}</span></div>
      <div class="mb-2 flex flex-wrap gap-2 text-xs">
      <span class="rounded-full border border-slate-700 bg-slate-800 px-2 py-1">${alert.severity}</span>
      <span class="rounded-full border border-blue-400/40 bg-blue-500/10 px-2 py-1 text-blue-300">${alert.status || 'received'}</span>
      <span class="rounded-full border px-2 py-1 ${badgeClassForDecision(decision)}">${decision}</span></div>
      <p class="text-sm text-slate-400">${alert.description}</p></article>`;
  }).join('');
  renderTimeline(alertsCache.find((a) => a.id === selectedAlertID));
}

function selectAlert(id) { selectedAlertID = id; loadAlertsPage(); }

function renderTimeline(alert) {
  const empty = document.getElementById('timeline-empty');
  const list = document.getElementById('timeline-list');
  const outcome = document.getElementById('triage-outcome');
  if (!alert || !alert.triage || !alert.triage.timeline) {
    empty.classList.remove('hidden'); list.classList.add('hidden'); outcome.classList.add('hidden');
    list.innerHTML = ''; outcome.innerHTML = ''; return;
  }
  empty.classList.add('hidden'); list.classList.remove('hidden');
  list.innerHTML = alert.triage.timeline.map((step) => `<li class="relative"><span class="absolute -left-[22px] top-1.5 h-2.5 w-2.5 rounded-full bg-cyan-400"></span><p class="text-sm font-medium">${step.phase} <span class="text-xs text-slate-500">· ${new Date(step.timestamp).toLocaleTimeString()}</span></p><p class="text-sm text-slate-400">${step.detail}</p></li>`).join('');
  const triage = alert.triage;
  outcome.classList.remove('hidden');
  outcome.innerHTML = `<h4 class="text-sm font-semibold text-cyan-300">Decision: ${triage.decision}</h4><p class="mt-2 text-sm text-slate-300">${triage.summary}</p><p class="mt-2 text-sm text-slate-300"><strong>Likely root cause:</strong> ${triage.likelyRootCause}</p>`;
}

async function loadIncidentsPage() {
  const incidents = await request('/api/incidents');
  document.getElementById('incidents-table').innerHTML = incidents.map((inc) => `<tr class="border-b border-slate-800"><td class="px-3 py-2">${inc.id}</td><td class="px-3 py-2">${inc.title}</td><td class="px-3 py-2">${inc.severity}</td><td class="px-3 py-2">${inc.status}</td></tr>`).join('') || '<tr><td colspan="4" class="px-3 py-4 text-slate-500">No incidents.</td></tr>';
}

function parseConfig(raw) {
  const config = {};
  raw.split('\n').map((line) => line.trim()).filter(Boolean).forEach((line) => {
    const [k, ...rest] = line.split('=');
    if (k && rest.length) config[k.trim()] = rest.join('=').trim();
  });
  return config;
}

function renderTools(tools) {
  const host = document.getElementById('tools-list');
  host.innerHTML = tools.map((tool) => `<article class="rounded-xl border border-slate-800 bg-slate-950 p-4"><div class="flex items-center justify-between"><h4 class="font-medium">${tool.name}</h4><span class="text-xs text-slate-500">${tool.id}</span></div><p class="text-sm text-slate-400">${tool.description}</p><p class="mt-2 text-xs text-slate-500">${tool.server} · ${tool.tool}</p><div class="mt-3 flex gap-2"><button onclick="editTool('${tool.id}')" class="rounded bg-slate-700 px-2 py-1 text-xs">Edit</button><button onclick="deleteTool('${tool.id}')" class="rounded bg-rose-600 px-2 py-1 text-xs">Delete</button></div></article>`).join('') || '<p class="text-sm text-slate-500">No MCP tools configured.</p>';
}

async function loadToolsPage() {
  toolsCache = await request('/api/tools');
  renderTools(toolsCache);
}

function editTool(id) {
  const tool = toolsCache.find((item) => item.id === id);
  if (!tool) return;
  editingToolID = id;
  document.getElementById('tool-name').value = tool.name;
  document.getElementById('tool-desc').value = tool.description;
  document.getElementById('tool-server').value = tool.server;
  document.getElementById('tool-tool').value = tool.tool;
  document.getElementById('tool-config').value = Object.entries(tool.config || {}).map(([k, v]) => `${k}=${v}`).join('\n');
  document.getElementById('tool-submit').textContent = 'Update Tool';
}

function resetToolForm() {
  editingToolID = '';
  document.getElementById('tool-form').reset();
  document.getElementById('tool-submit').textContent = 'Create Tool';
}

async function saveTool() {
  const payload = {
    name: document.getElementById('tool-name').value,
    description: document.getElementById('tool-desc').value,
    server: document.getElementById('tool-server').value,
    tool: document.getElementById('tool-tool').value,
    config: parseConfig(document.getElementById('tool-config').value)
  };
  if (!editingToolID) {
    await request('/api/tools', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
  } else {
    await request(`/api/tools/${editingToolID}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
  }
  resetToolForm();
  await loadToolsPage();
}

async function deleteTool(id) {
  await request(`/api/tools/${id}`, { method: 'DELETE' });
  await loadToolsPage();
}

async function loadUsers() { document.getElementById('users-out').textContent = JSON.stringify(await request('/api/admin/users'), null, 2); }
async function loadRoles() { document.getElementById('roles-out').textContent = JSON.stringify(await request('/api/admin/roles'), null, 2); }
async function loadInvites() { document.getElementById('invites-out').textContent = JSON.stringify(await request('/api/admin/invites'), null, 2); }

async function createUser() {
  const username = document.getElementById('new-username').value;
  const displayName = document.getElementById('new-display').value;
  const password = document.getElementById('new-password').value;
  const roles = document.getElementById('new-roles').value.split(',').map((s) => s.trim()).filter(Boolean);
  await request('/api/admin/users', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username, displayName, password, roles }) });
  await loadUsers();
}

async function createRole() {
  const name = document.getElementById('role-name').value;
  const description = document.getElementById('role-desc').value;
  const permissions = document.getElementById('role-perms').value.split(',').map((s) => s.trim()).filter(Boolean);
  await request('/api/admin/roles', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, description, permissions }) });
  await loadRoles();
}

async function createInvite() {
  const email = document.getElementById('invite-email').value;
  const role = document.getElementById('invite-role').value;
  await request('/api/admin/invites', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email, role }) });
  await loadInvites();
}

document.addEventListener('DOMContentLoaded', async () => {
  const page = document.body.dataset.page;
  if (page === 'statuspage') {
    await loadPublicStatusPage();
    return;
  }

  const root = document.getElementById('nav-root');
  if (root) root.innerHTML = navMarkup();
  await bootPage();
});
