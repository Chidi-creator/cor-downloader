const DEFAULT_URL = 'https://cor-downloader.onrender.com';

const apiUrlInput = document.getElementById('apiUrl');
const saveBtn = document.getElementById('saveBtn');
const resetBtn = document.getElementById('resetBtn');
const toast = document.getElementById('toast');

chrome.storage.sync.get({ apiUrl: DEFAULT_URL }, (data) => {
  apiUrlInput.value = data.apiUrl;
});

saveBtn.addEventListener('click', () => {
  const val = apiUrlInput.value.trim().replace(/\/$/, '');
  if (!val) {
    apiUrlInput.focus();
    return;
  }
  chrome.storage.sync.set({ apiUrl: val }, showToast);
});

resetBtn.addEventListener('click', () => {
  apiUrlInput.value = DEFAULT_URL;
  chrome.storage.sync.set({ apiUrl: DEFAULT_URL }, showToast);
});

apiUrlInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') saveBtn.click();
});

let toastTimer = null;
function showToast() {
  toast.classList.add('show');
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.classList.remove('show'), 2200);
}
