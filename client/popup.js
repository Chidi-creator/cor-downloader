let pollTimer = null;

const content = document.getElementById('content');
const urlInput = document.getElementById('urlText');

document.getElementById('settingsBtn').addEventListener('click', () => {
  chrome.runtime.openOptionsPage();
});

async function init() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab && tab.url) {
    urlInput.value = tab.url;
  }

  const { job } = await sendMessage({ type: 'GET_STATUS' });

  if (job && job.status && job.status !== 'idle') {
    renderState(job);
    if (job.status === 'submitting' || job.status === 'pending' || job.status === 'downloading') {
      startPolling();
    }
  } else {
    renderIdle();
  }
}

function renderIdle() {
  stopPolling();
  content.innerHTML = `
    <button class="btn-download" id="dlBtn">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
        <path d="M12 3v13M7 11l5 5 5-5" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
        <path d="M4 20h16" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"/>
      </svg>
      Download
    </button>
  `;
  document.getElementById('dlBtn').addEventListener('click', handleDownload);
}

function renderSubmitting() {
  content.innerHTML = `
    <div class="state-submitting">
      <div class="spinner"></div>
      <span class="state-label">Submitting job...</span>
    </div>
  `;
}

function renderDownloading(job) {
  const isPending = job.status === 'pending';
  const hasTotalBytes = job.total_bytes != null && job.total_bytes > 0;
  const downloaded = job.downloaded_bytes || 0;
  const total = job.total_bytes || 0;
  const pct = hasTotalBytes ? Math.min(100, Math.round((downloaded / total) * 100)) : 0;
  const isIndeterminate = isPending || downloaded === 0 || !hasTotalBytes;

  content.innerHTML = `
    <div class="state-downloading">
      <div class="progress-header">
        <span class="progress-label">
          <span class="pulse-dot"></span>
          Downloading
        </span>
        ${!isIndeterminate ? `<span class="progress-pct">${pct}%</span>` : ''}
      </div>
      <div class="progress-track">
        <div class="progress-fill ${isIndeterminate ? 'indeterminate' : ''}" style="width: ${isIndeterminate ? 40 : pct}%"></div>
      </div>
      <div class="progress-bytes">
        ${isPending
          ? 'Preparing...'
          : hasTotalBytes && downloaded > 0
            ? `${formatBytes(downloaded)} / ${formatBytes(total)}`
            : `${formatBytes(downloaded)} downloaded`
        }
      </div>
    </div>
  `;
}

function renderDone() {
  stopPolling();
  content.innerHTML = `
    <div class="state-done">
      <div class="icon-done">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
          <path d="M20 6L9 17l-5-5" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </div>
      <span class="done-text">Download complete!</span>
      <span class="done-sub">Check your Downloads folder</span>
    </div>
  `;
}

function renderFailed(job) {
  stopPolling();
  const msg = job.error || 'An unknown error occurred.';
  content.innerHTML = `
    <div class="state-failed">
      <div class="error-box">
        <span class="error-icon">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
            <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2"/>
            <path d="M12 8v4M12 16h.01" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </span>
        <span class="error-msg">${escHtml(msg)}</span>
      </div>
      <button class="btn-retry" id="retryBtn">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none">
          <path d="M1 4v6h6M23 20v-6h-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M20.49 9A9 9 0 005.64 5.64L1 10M23 14l-4.64 4.36A9 9 0 013.51 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        Try again
      </button>
    </div>
  `;
  document.getElementById('retryBtn').addEventListener('click', async () => {
    await sendMessage({ type: 'RESET' });
    renderIdle();
  });
}

function renderState(job) {
  if (!job) { renderIdle(); return; }
  switch (job.status) {
    case 'submitting': renderSubmitting(); break;
    case 'pending':
    case 'downloading': renderDownloading(job); break;
    case 'done': renderDone(); break;
    case 'failed': renderFailed(job); break;
    default: renderIdle();
  }
}

async function handleDownload() {
  const url = urlInput.value.trim();
  if (!url) {
    urlInput.focus();
    urlInput.placeholder = 'Paste a video URL first!';
    return;
  }

  const apiUrl = await getApiUrl();
  renderSubmitting();
  startPolling();
  await sendMessage({ type: 'START_DOWNLOAD', url, apiUrl });
}

function startPolling() {
  stopPolling();
  pollTimer = setInterval(async () => {
    const { job } = await sendMessage({ type: 'GET_STATUS' });
    if (!job) return;
    renderState(job);
    if (job.status === 'done' || job.status === 'failed') {
      stopPolling();
    }
  }, 200);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

async function getApiUrl() {
  return new Promise(resolve => {
    chrome.storage.sync.get({ apiUrl: 'https://cor-downloader.onrender.com' }, data => {
      resolve(data.apiUrl);
    });
  });
}

function sendMessage(msg) {
  return new Promise(resolve => {
    chrome.runtime.sendMessage(msg, response => {
      resolve(response || {});
    });
  });
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function escHtml(str) {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

init();
