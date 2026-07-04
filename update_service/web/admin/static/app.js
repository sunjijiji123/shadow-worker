const API = '';
let token = localStorage.getItem('update_token') || '';
let statusData = null;

function $(id) { return document.getElementById(id); }
function hide(id) { $(id).classList.add('hidden'); }
function show(id) { $(id).classList.remove('hidden'); }

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function formatBytes(bytes) {
    if (bytes === undefined || bytes === null) return '-';
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(2) + ' MB';
}

function formatDate(iso) {
    if (!iso) return '-';
    const d = new Date(iso);
    return d.toLocaleString('zh-CN', { dateStyle: 'long', timeStyle: 'short' });
}

function showToast(message, type = 'info') {
    const container = $('toast-container');
    const el = document.createElement('div');
    el.className = `toast ${type}`;
    el.innerHTML = `<span>${type === 'success' ? '✓' : type === 'error' ? '✕' : '•'}</span> <span>${escapeHtml(message)}</span>`;
    container.appendChild(el);
    setTimeout(() => {
        el.style.opacity = '0';
        el.style.transform = 'translateY(-10px)';
        el.style.transition = 'opacity 0.3s, transform 0.3s';
        setTimeout(() => el.remove(), 300);
    }, 3500);
}

async function api(path, opts = {}) {
    const url = (API || '') + path;
    const headers = {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': 'Bearer ' + token } : {}),
        ...opts.headers,
    };
    const res = await fetch(url, { ...opts, headers });
    if (res.status === 401) {
        token = '';
        localStorage.removeItem('update_token');
        showLogin();
        throw new Error('Session expired');
    }
    return res;
}

function showLogin() {
    hide('dashboard');
    $('dashboard').classList.remove('active');
    show('login-screen');
    $('login-screen').classList.add('active');
    $('login-error').textContent = '';
    $('password').value = '';
}

function showDashboard() {
    hide('login-screen');
    $('login-screen').classList.remove('active');
    show('dashboard');
    $('dashboard').classList.add('active');
}

function setLoginLoading(isLoading) {
    const btn = $('login-btn');
    btn.disabled = isLoading;
    btn.textContent = isLoading ? 'Signing in...' : 'Sign In';
}

async function login() {
    const username = $('username').value.trim();
    const password = $('password').value;
    const errorEl = $('login-error');
    errorEl.textContent = '';

    if (!username || !password) {
        errorEl.textContent = 'Please enter your credentials';
        return;
    }

    setLoginLoading(true);
    try {
        const res = await api('/api/auth/login', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) {
            throw new Error(data.error || `Access denied (${res.status})`);
        }
        token = data.token;
        localStorage.setItem('update_token', token);
        showDashboard();
        showToast('Welcome back', 'success');
        await refreshAll();
    } catch (e) {
        errorEl.textContent = e.message;
    } finally {
        setLoginLoading(false);
    }
}

function logout() {
    token = '';
    localStorage.removeItem('update_token');
    showLogin();
    showToast('Signed out', 'info');
}

async function loadStatus() {
    try {
        const res = await api('/admin/api/status');
        if (!res.ok) throw new Error('Failed to load status');
        statusData = await res.json();
        renderStatus(statusData);
    } catch (e) {
        console.error('loadStatus', e);
        $('status-grid').innerHTML = `<div class="info-block" style="grid-column:1/-1"><div class="value" style="color:var(--accent)">${escapeHtml(e.message)}</div></div>`;
    }
}

function renderStatus(data) {
    const repoUrl = `https://github.com/${encodeURIComponent(data.github_owner || '')}/${encodeURIComponent(data.github_repo || '')}`;
    const tokenBadge = data.github_token_configured
        ? `<span class="badge ok">Configured</span>`
        : `<span class="badge warn">Not configured</span>`;

    $('status-grid').innerHTML = `
        <div class="info-block">
            <div class="label">Repository</div>
            <div class="value"><a href="${repoUrl}/releases" target="_blank">${escapeHtml(data.github_owner + '/' + data.github_repo)}</a></div>
        </div>
        <div class="info-block">
            <div class="label">Token Status</div>
            <div class="value">${tokenBadge}</div>
        </div>
        <div class="info-block">
            <div class="label">Cache TTL</div>
            <div class="value">${escapeHtml(data.github_cache_ttl || '-')}</div>
        </div>
        <div class="info-block">
            <div class="label">Asset Template</div>
            <div class="value">${escapeHtml(data.asset_name_template || '-')}</div>
        </div>
    `;

    const releasesUrl = repoUrl + '/releases';
    const link = $('github-empty-link');
    if (link) link.href = releasesUrl;
}

async function loadReleases() {
    hide('releases-list');
    hide('releases-empty');
    hide('releases-error');
    show('releases-loading');

    try {
        const res = await api('/admin/api/releases?limit=100');
        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error(data.error || `Failed to load (${res.status})`);
        }
        const data = await res.json();
        const releases = data.releases || [];
        renderReleases(releases);
    } catch (e) {
        hide('releases-loading');
        hide('releases-empty');
        hide('releases-list');
        $('releases-error').textContent = e.message;
        show('releases-error');
        showToast(e.message, 'error');
    }
}

function renderReleases(releases) {
    hide('releases-loading');

    if (releases.length === 0) {
        hide('releases-list');
        show('releases-empty');
        return;
    }

    hide('releases-empty');
    const repoUrl = statusData
        ? `https://github.com/${encodeURIComponent(statusData.github_owner)}/${encodeURIComponent(statusData.github_repo)}`
        : '#';

    $('releases-list').innerHTML = releases.map(r => {
        const channelClass = r.channel === 'stable' ? 'stable' : 'beta';
        return `
            <div class="release-item">
                <div class="release-main">
                    <h3 class="release-title">Version ${escapeHtml(r.version)}</h3>
                    <span class="release-channel ${channelClass}">${escapeHtml(r.channel)}</span>
                    <span class="release-meta">${escapeHtml(r.package_filename)} · ${formatBytes(r.package_size)} · ${formatDate(r.published_at)}</span>
                </div>
                <div class="release-actions">
                    <a class="action-link primary" href="${escapeHtml(r.download_url)}" target="_blank" download>Download</a>
                    <a class="action-link" href="${escapeHtml(r.changelog_url || repoUrl + '/releases/tag/v' + r.version)}" target="_blank">Release Page</a>
                </div>
            </div>
        `;
    }).join('');
    show('releases-list');
}

async function refreshAll() {
    const btn = $('refresh-btn');
    const original = btn.textContent;
    btn.disabled = true;
    btn.textContent = 'Refreshing...';
    try {
        // 仪表盘只刷状态 + releases；配置数据进设置页时才加载
        await Promise.all([loadStatus(), loadReleases()]);
        showToast('Refreshed', 'success');
    } catch (e) {
        // handled in loaders
    } finally {
        btn.disabled = false;
        btn.textContent = original;
    }
}

// ============ View Switching（仪表盘内：主视图 ↔ 设置视图）============
// 注意：不要命名成 showDashboard，会和上面的登录屏切换函数冲突（同名覆盖）。
function showSettingsView() {
    $('dashboard-view').classList.add('hidden');
    $('settings-view').classList.remove('hidden');
    // 进入设置页默认显示 GitHub tab，并加载配置（懒加载）
    switchSettingsTab('github');
}

// 设置页内 tab 切换：github | account
function switchSettingsTab(tab) {
    const isGithub = tab === 'github';
    $('tab-github').classList.toggle('active', isGithub);
    $('tab-account').classList.toggle('active', !isGithub);
    $('settings-tab-github').classList.toggle('hidden', !isGithub);
    $('settings-tab-account').classList.toggle('hidden', isGithub);
    // 首次进 GitHub tab 时加载配置
    if (isGithub) loadConfig();
}

function showDashboardView() {
    $('settings-view').classList.add('hidden');
    $('dashboard-view').classList.remove('hidden');
}

// ============ GitHub Configuration ============
async function loadConfig() {
    const cfgLoading = $('config-loading');
    const cfgForm = $('config-form');
    cfgLoading.classList.remove('hidden');
    cfgForm.classList.add('hidden');
    try {
        const res = await api('/admin/api/config');
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Load config failed');
        cfgLoading.classList.add('hidden');
        cfgForm.classList.remove('hidden');

        $('cfg-owner').value = data.github_owner || '';
        $('cfg-repo').value = data.github_repo || '';
        $('cfg-token').value = '';  // 脱敏：永远不回显，留空=不修改
        $('cfg-token-status').textContent = data.github_token_masked
            ? `(configured: ${data.github_token_masked})`
            : '(not configured)';
        $('cfg-ttl').value = data.github_cache_ttl || '';
        $('cfg-asset').value = data.asset_name_template || '';
    } catch (e) {
        cfgLoading.classList.add('hidden');
        $('config-error').textContent = e.message;
        $('config-error').classList.remove('hidden');
    }
}

async function saveConfig(e) {
    e.preventDefault();
    const errEl = $('config-error');
    const btn = $('config-save-btn');
    errEl.classList.add('hidden');
    btn.disabled = true;
    btn.textContent = 'Saving...';
    try {
        const body = {
            github_owner: $('cfg-owner').value.trim(),
            github_repo: $('cfg-repo').value.trim(),
            github_cache_ttl: $('cfg-ttl').value.trim(),
            asset_name_template: $('cfg-asset').value.trim()
        };
        // token 留空=不修改；填了才提交
        const tokenVal = $('cfg-token').value;
        if (tokenVal) body.github_token = tokenVal;

        const res = await api('/admin/api/config', {
            method: 'PUT',
            body: JSON.stringify(body)
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Save failed');
        showToast('Configuration saved (hot-reloaded)', 'success');
        $('cfg-token').value = '';  // 清空，避免重复提交
        await loadConfig();  // 刷新脱敏显示
    } catch (e) {
        errEl.textContent = e.message;
        errEl.classList.remove('hidden');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Save Configuration';
    }
}

// ============ Change Password ============
async function changePassword(e) {
    e.preventDefault();
    const errEl = $('password-error');
    const btn = $('password-save-btn');
    errEl.classList.add('hidden');

    const oldPw = $('pw-old').value;
    const newPw = $('pw-new').value;
    const confirmPw = $('pw-confirm').value;
    if (newPw !== confirmPw) {
        errEl.textContent = 'New passwords do not match';
        errEl.classList.remove('hidden');
        return;
    }
    if (newPw.length < 6) {
        errEl.textContent = 'New password must be at least 6 characters';
        errEl.classList.remove('hidden');
        return;
    }

    btn.disabled = true;
    btn.textContent = 'Changing...';
    try {
        const res = await api('/admin/api/password', {
            method: 'POST',
            body: JSON.stringify({
                old_password: oldPw,
                new_password: newPw
            })
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Change failed');
        showToast('Password changed. Use the new password next login.', 'success');
        $('pw-old').value = '';
        $('pw-new').value = '';
        $('pw-confirm').value = '';
    } catch (e) {
        errEl.textContent = e.message;
        errEl.classList.remove('hidden');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Change Password';
    }
}

// Bind form submits
$('config-form').addEventListener('submit', saveConfig);
$('password-form').addEventListener('submit', changePassword);

// Initialize
if (token) {
    showDashboard();
    refreshAll();
} else {
    showLogin();
}
