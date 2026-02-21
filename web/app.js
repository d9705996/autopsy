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
      description: 'Timeout errors above SLO objective',
      severity: 'critical',
      labels: {service: 'api', metric: 'http_5xx_rate'}
    })
  });
  await loadData();
}

async function loadData() {
  const [alerts, incidents, postmortems, playbooks, oncall] = await Promise.all([
    request('/api/alerts'),
    request('/api/incidents'),
    request('/api/postmortems'),
    request('/api/playbooks'),
    request('/api/oncall')
  ]);
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
  const roles = document.getElementById('new-roles').value.split(',').map(s => s.trim()).filter(Boolean);
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
  const permissions = document.getElementById('role-perms').value.split(',').map(s => s.trim()).filter(Boolean);
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
