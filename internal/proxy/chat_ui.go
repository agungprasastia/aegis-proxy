package proxy

import (
	"fmt"
	"net/http"
)

func (ps *ProxyServer) handleChatUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, chatUIHTML)
}

const chatUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Aegis Chat</title>
  <style>
    :root { color-scheme: dark; }
    body { margin: 0; font-family: system-ui, sans-serif; background: #0b1020; color: #e5e7eb; }
    .wrap { max-width: 980px; margin: 0 auto; padding: 20px; display: grid; gap: 16px; }
    .top { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
    .card { background: #111827; border: 1px solid #243041; border-radius: 14px; padding: 14px; }
    #log { min-height: 60vh; max-height: 68vh; overflow: auto; display: grid; gap: 12px; }
    .msg { white-space: pre-wrap; line-height: 1.45; padding: 12px 14px; border-radius: 12px; }
    .user { background: #1d4ed8; justify-self: end; max-width: 85%; }
    .assistant { background: #1f2937; max-width: 85%; }
    .meta { font-size: 12px; opacity: .75; margin-bottom: 6px; }
    .row { display: grid; grid-template-columns: 1fr auto; gap: 10px; }
    textarea, input, select, button { font: inherit; }
    textarea { width: 100%; min-height: 78px; resize: vertical; border-radius: 12px; border: 1px solid #334155; background: #0f172a; color: #e5e7eb; padding: 12px; box-sizing: border-box; }
    button, select, input { border-radius: 10px; border: 1px solid #334155; background: #0f172a; color: #e5e7eb; padding: 10px 12px; }
    button { cursor: pointer; }
    button:disabled { opacity: .6; cursor: not-allowed; }
    .small { font-size: 12px; opacity: .8; }
    .status { font-size: 13px; color: #93c5fd; }
    code { background: #0f172a; padding: 2px 6px; border-radius: 6px; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="top">
      <div>
        <h1 style="margin:0">Aegis Chat</h1>
        <div class="small">Built-in UI for <code>/v1/chat/completions</code></div>
      </div>
      <div class="status" id="status">Ready</div>
    </div>

    <div class="card">
      <div class="row" style="margin-bottom:10px">
        <input id="model" value="gpt-4" placeholder="Model" />
        <label style="display:flex;align-items:center;gap:8px;padding:10px 0">
          <input id="stream" type="checkbox" checked style="width:auto" /> Stream
        </label>
      </div>
      <textarea id="prompt" placeholder="Type a message and send..."></textarea>
      <div class="row" style="margin-top:10px">
        <div class="small">Sends requests to this proxy itself.</div>
        <button id="send">Send</button>
      </div>
    </div>

    <div class="card">
      <div id="log"></div>
    </div>
  </div>

<script>
const log = document.getElementById('log');
const promptEl = document.getElementById('prompt');
const modelEl = document.getElementById('model');
const streamEl = document.getElementById('stream');
const statusEl = document.getElementById('status');
const sendBtn = document.getElementById('send');
const messages = [];

function addMessage(role, text) {
  const wrap = document.createElement('div');
  wrap.className = 'msg ' + role;
  const meta = document.createElement('div');
  meta.className = 'meta';
  meta.textContent = role === 'user' ? 'You' : 'Assistant';
  const body = document.createElement('div');
  body.textContent = text || '';
  wrap.appendChild(meta);
  wrap.appendChild(body);
  log.appendChild(wrap);
  log.scrollTop = log.scrollHeight;
  return body;
}

function setStatus(text) { statusEl.textContent = text; }

function buildPayload(stream) {
  return {
    model: modelEl.value || 'gpt-4',
    messages: messages.map(m => ({ role: m.role, content: m.content })),
    stream,
  };
}

async function send() {
  const content = promptEl.value.trim();
  if (!content) return;

  messages.push({ role: 'user', content });
  addMessage('user', content);
  promptEl.value = '';

  const assistantBody = addMessage('assistant', '');
  messages.push({ role: 'assistant', content: '' });
  sendBtn.disabled = true;
  setStatus('Sending...');

  try {
    const res = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(buildPayload(streamEl.checked)),
    });

    if (!res.ok) {
      throw new Error(await res.text() || ('HTTP ' + res.status));
    }

    if (!streamEl.checked) {
      const data = await res.json();
      const text = data?.choices?.[0]?.message?.content || '';
      assistantBody.textContent = text;
      messages[messages.length - 1].content = text;
      setStatus('Done');
      return;
    }

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let full = '';

    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let idx;
      while ((idx = buffer.indexOf('\n\n')) !== -1) {
        const chunk = buffer.slice(0, idx).trim();
        buffer = buffer.slice(idx + 2);
        if (!chunk) continue;
        for (const line of chunk.split('\n')) {
          if (!line.startsWith('data:')) continue;
          const data = line.slice(5).trim();
          if (data === '[DONE]') continue;
          try {
            const parsed = JSON.parse(data);
            const delta = parsed?.choices?.[0]?.delta?.content || '';
            if (delta) {
              full += delta;
              assistantBody.textContent = full;
              messages[messages.length - 1].content = full;
              log.scrollTop = log.scrollHeight;
            }
          } catch (e) {}
        }
      }
    }

    setStatus('Done');
  } catch (err) {
    assistantBody.textContent = 'Error: ' + (err?.message || err);
    messages[messages.length - 1].content = 'Error';
    setStatus('Error');
  } finally {
    sendBtn.disabled = false;
  }
}

sendBtn.addEventListener('click', send);
promptEl.addEventListener('keydown', (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') send();
});
</script>
</body>
</html>`
