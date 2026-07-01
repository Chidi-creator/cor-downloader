let currentJob = null;
let pollingInterval = null;

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'START_DOWNLOAD') {
    startDownload(message.url, message.apiUrl)
      .then(() => sendResponse({ ok: true }))
      .catch(err => sendResponse({ ok: false, error: err.message }));
    return true;
  }

  if (message.type === 'GET_STATUS') {
    sendResponse({ job: currentJob });
    return true;
  }

  if (message.type === 'RESET') {
    stopPolling();
    currentJob = null;
    sendResponse({ ok: true });
    return true;
  }
});

async function startDownload(url, apiUrl) {
  stopPolling();
  currentJob = { status: 'submitting', apiUrl, url };

  try {
    const res = await fetch(`${apiUrl}/jobs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url })
    });

    if (!res.ok) {
      const text = await res.text().catch(() => `HTTP ${res.status}`);
      throw new Error(text || `Server returned ${res.status}`);
    }

    const job = await res.json();
    currentJob = { ...job, apiUrl };
    startPolling(job.id, apiUrl);
  } catch (err) {
    currentJob = { status: 'failed', error: err.message, apiUrl, url };
  }
}

function startPolling(jobId, apiUrl) {
  stopPolling();

  pollingInterval = setInterval(async () => {
    try {
      const res = await fetch(`${apiUrl}/jobs/${jobId}`);
      if (!res.ok) throw new Error(`Poll failed: HTTP ${res.status}`);

      const job = await res.json();
      currentJob = { ...job, apiUrl };

      if (job.status === 'done') {
        stopPolling();
        triggerChromeDownload(jobId, apiUrl);
      } else if (job.status === 'failed') {
        stopPolling();
      }
    } catch (err) {
      stopPolling();
      currentJob = { status: 'failed', error: err.message, apiUrl };
    }
  }, 500);
}

function triggerChromeDownload(jobId, apiUrl) {
  const fileUrl = `${apiUrl}/jobs/${jobId}/file`;

  currentJob = { ...currentJob, status: 'downloading', downloaded_bytes: 0, total_bytes: null };

  chrome.downloads.download({ url: fileUrl }, (downloadId) => {
    if (chrome.runtime.lastError) {
      currentJob = { ...currentJob, status: 'failed', error: chrome.runtime.lastError.message };
      return;
    }

    // Poll Chrome's own download item for real byte progress
    const progressInterval = setInterval(() => {
      chrome.downloads.search({ id: downloadId }, ([item]) => {
        if (!item) return;
        currentJob = {
          ...currentJob,
          downloaded_bytes: item.bytesReceived,
          total_bytes: item.totalBytes > 0 ? item.totalBytes : null,
        };
      });
    }, 200);

    function onChanged(delta) {
      if (delta.id !== downloadId) return;
      if (!delta.state) return;

      if (delta.state.current === 'complete') {
        clearInterval(progressInterval);
        chrome.downloads.onChanged.removeListener(onChanged);
        currentJob = { ...currentJob, status: 'done', downloadId };
      } else if (delta.state.current === 'interrupted') {
        clearInterval(progressInterval);
        chrome.downloads.onChanged.removeListener(onChanged);
        currentJob = { ...currentJob, status: 'failed', error: 'Download was interrupted' };
      }
    }

    chrome.downloads.onChanged.addListener(onChanged);
  });
}

function stopPolling() {
  if (pollingInterval) {
    clearInterval(pollingInterval);
    pollingInterval = null;
  }
}
