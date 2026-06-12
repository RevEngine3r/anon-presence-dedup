// ── STATE ────────────────────────────────────────────────────────────────────
let state = {
  channel: null,
  messages: [],
  isAdmin: false,
  adminToken: null,
  clientId: getOrCreateClientId(),
  onlineCount: 1,
  seenViews: new Set(),
  ws: null,
};

// ── CLIENT ID (anon dedup) ────────────────────────────────────────────────────
function getOrCreateClientId() {
  let id;
  try { id = localStorage.getItem('client_id'); } catch(e) {}
  if (!id) {
    id = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
      const r = Math.random() * 16 | 0;
      return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
    });
    try { localStorage.setItem('client_id', id); } catch(e) {}
  }
  return id;
}

// ── API HELPERS ───────────────────────────────────────────────────────────────
async function api(method, path, body) {
  const headers = {
    'Content-Type': 'application/json',
    'X-Client-ID': state.clientId,
  };
  if (state.adminToken) headers['X-Admin-Token'] = state.adminToken;
  try {
    const r = await fetch(API_BASE + path, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
    if (r.status === 204) return {};
    return await r.json();
  } catch (e) {
    console.warn('API error:', e);
    return null;
  }
}

// ── INIT ──────────────────────────────────────────────────────────────────────
async function init() {
  try {
    const tok = localStorage.getItem('admin_token');
    if (tok) { state.adminToken = tok; state.isAdmin = true; }
  } catch(e) {}

  applyAdminUI();
  await loadChannel();
  await loadMessages();
  setupWebSocket();
  setupIntersectionObserver();
  setupThemeToggle();
}

// ── CHANNEL ──────────────────────────────────────────────────────────────────
async function loadChannel() {
  const channels = await api('GET', '/channels');
  if (!channels || !channels.length) return;
  state.channel = channels[0];
  renderChannelInfo();
}

function renderChannelInfo() {
  const ch = state.channel;
  if (!ch) return;

  const name = ch.name || 'Channel';
  const desc = ch.description || '';
  const emoji = ch.emoji || '💬';
  const logoUrl = ch.logo_url || null;

  document.getElementById('sidebar-name').textContent = name;
  document.getElementById('sidebar-desc').textContent = desc;
  document.getElementById('sidebar-emoji').textContent = emoji;
  document.getElementById('header-emoji').textContent = emoji;
  document.getElementById('header-name').textContent = name;
  document.title = name;

  const sidebarAvatar = document.getElementById('sidebar-avatar');
  const headerAvatar = document.getElementById('header-avatar');
  if (logoUrl) {
    sidebarAvatar.innerHTML = `<img src="${escHtml(logoUrl)}" alt="${escHtml(name)} logo" width="80" height="80" loading="lazy">`;
    headerAvatar.innerHTML = `<img src="${escHtml(logoUrl)}" alt="" width="36" height="36" loading="lazy" style="width:100%;height:100%;object-fit:cover;border-radius:50%;">`;
  } else {
    sidebarAvatar.innerHTML = `<span id="sidebar-emoji">${emoji}</span>`;
    headerAvatar.innerHTML = `<span id="header-emoji">${emoji}</span>`;
  }
}

// ── MESSAGES ──────────────────────────────────────────────────────────────────
async function loadMessages() {
  if (!state.channel) {
    hideSkeletons();
    return;
  }
  const msgs = await api('GET', `/channels/${state.channel.id}/messages`);
  hideSkeletons();
  if (!msgs) return;
  state.messages = msgs;
  renderMessages();
  scrollToBottom(false);
}

function hideSkeletons() {
  const sk = document.getElementById('loading-skeletons');
  if (sk) sk.style.display = 'none';
}

function renderMessages() {
  const container = document.getElementById('messages-container');
  const empty = document.getElementById('empty-state');
  const msgs = state.messages;

  if (!msgs.length) {
    container.innerHTML = '';
    empty.style.display = 'flex';
    document.getElementById('stat-msgs').textContent = '0';
    return;
  }

  empty.style.display = 'none';
  document.getElementById('stat-msgs').textContent = msgs.length;

  let html = '';
  let lastDate = '';

  msgs.forEach(msg => {
    const d = new Date(msg.created_at);
    const dateStr = d.toLocaleDateString('en-US', {month:'long',day:'numeric',year:'numeric'});
    if (dateStr !== lastDate) {
      html += `<div class="date-separator"><span class="date-label">${escHtml(dateStr)}</span></div>`;
      lastDate = dateStr;
    }
    html += renderMessage(msg);
  });

  container.innerHTML = html;
}

function renderMessage(msg) {
  const time = new Date(msg.created_at).toLocaleTimeString('en-US', {hour:'2-digit',minute:'2-digit',hour12:false});
  const views = msg.view_count || 0;
  const reactions = msg.reactions || {};
  const adminControls = state.isAdmin ? `
    <div class="msg-admin-controls" role="toolbar" aria-label="Message actions">
      <button class="msg-ctrl-btn" onclick="openEditMsg(${msg.id})" title="Edit" aria-label="Edit message">✏️</button>
      <button class="msg-ctrl-btn del" onclick="openDeleteMsg(${msg.id})" title="Delete" aria-label="Delete message">🗑️</button>
    </div>` : '';

  const reactionsHtml = Object.entries(reactions).filter(([,c])=>c>0).map(([emoji,count]) =>
    `<button class="reaction-chip" onclick="reactTo(${msg.id},'${escHtml(emoji)}')" title="React with ${escHtml(emoji)}" aria-label="${escHtml(emoji)}: ${count}">
      <span class="emoji">${escHtml(emoji)}</span>
      <span class="count">${count}</span>
    </button>`
  ).join('');

  const emojiPanel = `<div class="emoji-panel" id="ep-${msg.id}" role="group" aria-label="Quick reactions">
    ${EMOJIS.map(e=>`<button onclick="reactTo(${msg.id},'${e}');closeEmojiPanel(${msg.id})" aria-label="${e}">${e}</button>`).join('')}
  </div>`;

  return `
  <div class="message-wrap" data-msg-id="${msg.id}">
    <div class="message-bubble ${msg.image_url?'has-image':''}">
      ${adminControls}
      ${msg.image_url ? `<img class="msg-image" src="${escHtml(msg.image_url)}" alt="Message image" loading="lazy" onclick="openImage('${escHtml(msg.image_url)}')" width="320">` : ''}
      ${msg.content ? `<div class="msg-text">${escHtml(msg.content)}</div>` : ''}
      <div class="msg-meta">
        ${!state.isAdmin ? `<button class="reaction-chip" style="background:transparent;border-color:transparent;padding:0 2px;" onclick="toggleEmojiPanel(${msg.id})" title="React" aria-label="Add reaction">😊</button>` : ''}
        <span class="msg-views">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
          ${views}
        </span>
        <span class="msg-time">${time}</span>
      </div>
    </div>
    ${emojiPanel}
    ${reactionsHtml ? `<div class="reactions-row">${reactionsHtml}</div>` : ''}
  </div>`;
}

function scrollToBottom(smooth=true) {
  const feed = document.getElementById('messages-feed');
  feed.scrollTo({ top: feed.scrollHeight, behavior: smooth ? 'smooth' : 'instant' });
}

// ── VIEW TRACKING (dedup via X-Client-ID) ────────────────────────────────────
let viewObserver;
function setupIntersectionObserver() {
  viewObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (!entry.isIntersecting) return;
      const el = entry.target;
      const msgId = el.dataset.msgId;
      if (!msgId || state.seenViews.has(msgId)) return;
      state.seenViews.add(msgId);
      api('POST', `/messages/${msgId}/view`);
      viewObserver.unobserve(el);
    });
  }, { threshold: 0.5 });
  observeMessages();
}

function observeMessages() {
  if (!viewObserver) return;
  document.querySelectorAll('[data-msg-id]').forEach(el => viewObserver.observe(el));
}

// ── REACTIONS ─────────────────────────────────────────────────────────────────
function toggleEmojiPanel(msgId) {
  const panel = document.getElementById(`ep-${msgId}`);
  if (!panel) return;
  const isOpen = panel.classList.contains('open');
  document.querySelectorAll('.emoji-panel.open').forEach(p => p.classList.remove('open'));
  if (!isOpen) panel.classList.add('open');
}

function closeEmojiPanel(msgId) {
  const panel = document.getElementById(`ep-${msgId}`);
  if (panel) panel.classList.remove('open');
}

async function reactTo(msgId, emoji) {
  const res = await api('POST', `/messages/${msgId}/react`, { emoji });
  if (res && res.ok) {
    const msg = state.messages.find(m => m.id === msgId);
    if (msg) {
      if (!msg.reactions) msg.reactions = {};
      msg.reactions[emoji] = (msg.reactions[emoji] || 0) + 1;
      updateMessageReactions(msgId, msg.reactions);
    }
  }
}

function updateMessageReactions(msgId, reactions) {
  const wrap = document.querySelector(`[data-msg-id="${msgId}"]`);
  if (!wrap) return;
  let reactRow = wrap.querySelector('.reactions-row');
  const reactionsHtml = Object.entries(reactions).filter(([,c])=>c>0).map(([emoji,count]) =>
    `<button class="reaction-chip" onclick="reactTo(${msgId},'${escHtml(emoji)}')" aria-label="${escHtml(emoji)}: ${count}">
      <span class="emoji">${escHtml(emoji)}</span>
      <span class="count">${count}</span>
    </button>`
  ).join('');
  if (!reactRow) {
    reactRow = document.createElement('div');
    reactRow.className = 'reactions-row';
    wrap.appendChild(reactRow);
  }
  reactRow.innerHTML = reactionsHtml;
}

// ── WEBSOCKET ─────────────────────────────────────────────────────────────────
function setupWebSocket() {
  const wsProto = location.protocol === 'https:' ? 'wss' : 'ws';
  const wsUrl = `${wsProto}://${location.host}/ws?client_id=${state.clientId}`;
  try {
    state.ws = new WebSocket(wsUrl);
    state.ws.onopen = () => {
      const bar = document.getElementById('presence-bar');
      bar.classList.add('visible');
      document.getElementById('presence-text').textContent = 'Connected · Live updates active';
    };
    state.ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        handleWSMessage(msg);
      } catch(_) {}
    };
    state.ws.onclose = () => {
      const bar = document.getElementById('presence-bar');
      bar.classList.remove('visible');
      setTimeout(setupWebSocket, 3000);
    };
    state.ws.onerror = () => { state.ws.close(); };
  } catch(e) {}
}

function handleWSMessage(msg) {
  if (msg.type === 'PRESENCE_UPDATE') {
    state.onlineCount = msg.online || 1;
    document.getElementById('stat-online').textContent = state.onlineCount;
    document.getElementById('presence-text').textContent = `${state.onlineCount} online · Live`;
    document.getElementById('header-sub').textContent = `${state.onlineCount} online`;
  }
  if (msg.type === 'NEW_MESSAGE' || msg.type === 'MESSAGE_UPDATED' || msg.type === 'MESSAGE_DELETED') {
    loadMessages();
  }
}

// ── ADMIN LOGIN ───────────────────────────────────────────────────────────────
async function loginSubmit() {
  const user = document.getElementById('login-user').value.trim();
  const pass = document.getElementById('login-pass').value;
  const err  = document.getElementById('login-error');
  const btn  = document.getElementById('login-submit-btn');

  if (!user || !pass) { err.textContent = 'Please enter username and password.'; err.classList.add('visible'); return; }

  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span>';
  err.classList.remove('visible');

  const res = await api('POST', '/admin/login', { username: user, password: pass });

  btn.disabled = false;
  btn.innerHTML = 'Sign In';

  if (!res || !res.token) {
    err.textContent = res?.error || 'Invalid credentials.';
    err.classList.add('visible');
    return;
  }

  state.adminToken = res.token;
  state.isAdmin = true;
  try { localStorage.setItem('admin_token', res.token); } catch(e) {}

  closeModal('login-modal');
  applyAdminUI();
  showToast('✓ Admin logged in');
  renderMessages();
  observeMessages();
}

function adminLogout() {
  state.adminToken = null;
  state.isAdmin = false;
  try { localStorage.removeItem('admin_token'); } catch(e) {}
  applyAdminUI();
  renderMessages();
  showToast('Logged out');
}

function applyAdminUI() {
  const isAdmin = state.isAdmin;
  document.getElementById('admin-badge').style.display = isAdmin ? 'flex' : 'none';
  document.getElementById('login-btn').style.display = isAdmin ? 'none' : 'flex';
  document.getElementById('admin-panel').classList.toggle('visible', isAdmin);
  document.getElementById('compose-bar').classList.toggle('visible', isAdmin);
}

// ── CHANNEL EDIT ─────────────────────────────────────────────────────────────
function openEditChannelModal() {
  const ch = state.channel;
  if (!ch) return;
  document.getElementById('ec-name').value = ch.name || '';
  document.getElementById('ec-desc').value = ch.description || '';
  document.getElementById('ec-emoji').value = ch.emoji || '💬';
  openModal('edit-channel-modal');
}

async function saveChannelEdit() {
  const name = document.getElementById('ec-name').value.trim();
  const desc = document.getElementById('ec-desc').value.trim();
  const emoji = document.getElementById('ec-emoji').value.trim();
  if (!name) { showToast('Channel name is required'); return; }
  if (!state.channel) return;

  const res = await api('PATCH', `/channels/${state.channel.id}`, { name, description: desc, emoji: emoji || '💬' });
  if (res && res.ok) {
    state.channel.name = name;
    state.channel.description = desc;
    state.channel.emoji = emoji || '💬';
    renderChannelInfo();
    closeModal('edit-channel-modal');
    showToast('✓ Channel updated');
  } else {
    showToast('Failed to save changes');
  }
}

// ── LOGO UPDATE ───────────────────────────────────────────────────────────────
function openLogoModal() {
  document.getElementById('logo-url-input').value = state.channel?.logo_url || '';
  document.getElementById('logo-preview-wrap').style.display = 'none';
  document.getElementById('logo-modal').classList.add('open');
}

async function saveLogo() {
  const url = document.getElementById('logo-url-input').value.trim();
  if (!url) { showToast('Please enter an image URL'); return; }
  const res = await api('PATCH', `/channels/${state.channel.id}`, { logo_url: url });
  if (res && res.ok) {
    state.channel.logo_url = url;
    renderChannelInfo();
    closeModal('logo-modal');
    showToast('✓ Logo updated');
  } else { showToast('Failed to update logo'); }
}

async function removeLogo() {
  const res = await api('PATCH', `/channels/${state.channel.id}`, { logo_url: '' });
  if (res && res.ok) {
    state.channel.logo_url = null;
    renderChannelInfo();
    closeModal('logo-modal');
    showToast('Logo removed');
  } else { showToast('Failed to remove logo'); }
}

// ── SEND MESSAGE ──────────────────────────────────────────────────────────────
async function sendMessage() {
  if (!state.isAdmin || !state.channel) return;
  const textarea = document.getElementById('compose-textarea');
  const imgInput = document.getElementById('img-url-input');
  const content = textarea.value.trim();
  const imageUrl = imgInput.value.trim() || undefined;

  if (!content && !imageUrl) { showToast('Write something or add an image'); return; }

  textarea.value = '';
  imgInput.value = '';
  autoResize(textarea);
  hideImgUrl();

  const res = await api('POST', `/channels/${state.channel.id}/messages`, { content, image_url: imageUrl || null });
  if (res && res.id) {
    state.messages.push(res);
    renderMessages();
    observeMessages();
    scrollToBottom(true);
  } else { showToast('Failed to send message'); }
}

function handleComposeKey(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
}

function autoResize(el) {
  el.style.height = 'auto';
  el.style.height = Math.min(el.scrollHeight, 120) + 'px';
}

function toggleImgUrl() {
  const row = document.getElementById('img-url-row');
  row.classList.toggle('visible');
  if (row.classList.contains('visible')) {
    document.getElementById('img-url-input').focus();
  }
}

function hideImgUrl() {
  document.getElementById('img-url-row').classList.remove('visible');
}

function clearImgUrl() {
  document.getElementById('img-url-input').value = '';
  hideImgUrl();
}

// ── EDIT MESSAGE ──────────────────────────────────────────────────────────────
function openEditMsg(msgId) {
  const msg = state.messages.find(m => m.id === msgId);
  if (!msg) return;
  document.getElementById('edit-msg-id').value = msgId;
  document.getElementById('edit-msg-content').value = msg.content || '';
  document.getElementById('edit-msg-img').value = msg.image_url || '';
  openModal('edit-msg-modal');
}

async function saveMessageEdit() {
  const msgId = parseInt(document.getElementById('edit-msg-id').value);
  const content = document.getElementById('edit-msg-content').value.trim();
  const imageUrl = document.getElementById('edit-msg-img').value.trim();

  const res = await api('PATCH', `/messages/${msgId}`, {
    content: content || undefined,
    image_url: imageUrl || null,
  });

  if (res && res.ok) {
    const idx = state.messages.findIndex(m => m.id === msgId);
    if (idx !== -1) {
      state.messages[idx].content = content;
      state.messages[idx].image_url = imageUrl || null;
    }
    renderMessages();
    observeMessages();
    closeModal('edit-msg-modal');
    showToast('✓ Message updated');
  } else { showToast('Failed to update message'); }
}

// ── DELETE MESSAGE ────────────────────────────────────────────────────────────
function openDeleteMsg(msgId) {
  document.getElementById('confirm-action-id').value = msgId;
  document.getElementById('confirm-title').textContent = '🗑️ Delete Message';
  openModal('confirm-modal');
}

async function executeDelete() {
  const val = document.getElementById('confirm-action-id').value;
  if (val === 'CLEAR') {
    const res = await api('DELETE', `/channels/${state.channel.id}/messages`);
    if (res !== null) {
      state.messages = [];
      renderMessages();
      closeModal('confirm-modal');
      showToast('All messages cleared');
    } else { showToast('Failed to clear messages'); }
  } else {
    const msgId = parseInt(val);
    const res = await api('DELETE', `/messages/${msgId}`);
    if (res !== null) {
      state.messages = state.messages.filter(m => m.id !== msgId);
      renderMessages();
      observeMessages();
      closeModal('confirm-modal');
      showToast('Message deleted');
    } else { showToast('Failed to delete message'); }
  }
}

async function confirmClear() {
  if (!state.channel) return;
  document.getElementById('confirm-action-id').value = 'CLEAR';
  document.getElementById('confirm-title').textContent = '🗑️ Clear All Messages';
  document.querySelector('#confirm-modal p').textContent = 'This will permanently delete ALL messages in this channel. This cannot be undone.';
  openModal('confirm-modal');
}

// ── IMAGE VIEWER ──────────────────────────────────────────────────────────────
function openImage(url) {
  document.getElementById('image-modal-img').src = url;
  openModal('image-modal');
}

// ── SIDEBAR ───────────────────────────────────────────────────────────────────
function openSidebar() {
  document.getElementById('sidebar').classList.add('open');
  document.getElementById('sidebar-backdrop').classList.add('open');
  if (state.channel) {
    document.getElementById('ec-name').value = state.channel.name || '';
    document.getElementById('ec-desc').value = state.channel.description || '';
  }
}

function closeSidebar() {
  document.getElementById('sidebar').classList.remove('open');
  document.getElementById('sidebar-backdrop').classList.remove('open');
}

// ── MODALS ────────────────────────────────────────────────────────────────────
function openModal(id) {
  const el = document.getElementById(id);
  if (!el) return;
  if (id === 'edit-channel-modal') { _openEditChannelModalDirect(); return; }
  if (id === 'logo-modal') { openLogoModal(); return; }
  el.classList.add('open');
  const firstInput = el.querySelector('input, textarea');
  if (firstInput) setTimeout(() => firstInput.focus(), 250);
}

function closeModal(id) {
  const el = document.getElementById(id);
  if (el) el.classList.remove('open');
}

function _openEditChannelModalDirect() {
  if (state.channel) {
    document.getElementById('ec-name').value = state.channel.name || '';
    document.getElementById('ec-desc').value = state.channel.description || '';
    document.getElementById('ec-emoji').value = state.channel.emoji || '💬';
  }
  document.getElementById('edit-channel-modal').classList.add('open');
}

// ── THEME TOGGLE ──────────────────────────────────────────────────────────────
function setupThemeToggle() {
  const stored = (() => { try { return localStorage.getItem('theme'); } catch(e) {} })();
  const sysDark = matchMedia('(prefers-color-scheme: dark)').matches;
  const theme = stored || (sysDark ? 'dark' : 'light');
  setTheme(theme);

  document.querySelectorAll('[data-theme-toggle]').forEach(btn => {
    btn.addEventListener('click', () => {
      const cur = document.documentElement.getAttribute('data-theme') || 'dark';
      setTheme(cur === 'dark' ? 'light' : 'dark');
    });
  });
}

function setTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
  try { localStorage.setItem('theme', theme); } catch(e) {}
  const moonSvg = `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>`;
  const sunSvg  = `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>`;
  document.querySelectorAll('[data-theme-toggle]').forEach(btn => {
    btn.innerHTML = theme === 'dark' ? moonSvg : sunSvg;
    btn.setAttribute('aria-label', `Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`);
  });
}

// ── TOAST ─────────────────────────────────────────────────────────────────────
let toastTimer;
function showToast(msg) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.classList.add('show');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.remove('show'), 2800);
}

// ── UTILS ─────────────────────────────────────────────────────────────────────
function escHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
    .replace(/"/g,'&quot;').replace(/'/g,'&#039;');
}

// ── EVENT LISTENERS ───────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
  // Logo preview
  document.getElementById('logo-url-input').addEventListener('input', function() {
    const url = this.value.trim();
    const preview = document.getElementById('logo-preview-wrap');
    const img = document.getElementById('logo-preview-img');
    if (url) {
      img.src = url;
      preview.style.display = 'block';
    } else {
      preview.style.display = 'none';
    }
  });

  // Close modals on overlay click
  document.querySelectorAll('.modal-overlay').forEach(overlay => {
    overlay.addEventListener('click', function(e) {
      if (e.target === this && this.id !== 'image-modal') closeModal(this.id);
    });
  });

  // Escape key closes modals & emoji panels
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
      document.querySelectorAll('.modal-overlay.open').forEach(m => m.classList.remove('open'));
      document.querySelectorAll('.emoji-panel.open').forEach(p => p.classList.remove('open'));
    }
  });

  // Close emoji panels when clicking outside
  document.addEventListener('click', e => {
    if (!e.target.closest('.emoji-panel') && !e.target.closest('.reaction-chip')) {
      document.querySelectorAll('.emoji-panel.open').forEach(p => p.classList.remove('open'));
    }
  });

  // Rewire admin action buttons
  document.querySelectorAll('.admin-action-btn').forEach(btn => {
    const onclick = btn.getAttribute('onclick');
    if (onclick && onclick.includes('edit-channel-modal')) {
      btn.setAttribute('onclick', '_openEditChannelModalDirect()');
    }
    if (onclick && onclick.includes('logo-modal')) {
      btn.setAttribute('onclick', 'openLogoModal()');
    }
  });

  init();
});
