<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from './api.js';
  import Badge from './components/Badge.svelte';

  let { onback = () => {} } = $props();

  let status = $state(null);
  let loading = $state(true);
  let pollInterval;
  let eventSource;

  async function fetchStatus() {
    try {
      const res = await api.get('/api/batch/status');
      status = res;
      loading = false;

      // Stop polling if completed/cancelled/idle
      if (res && (res.status === 'completed' || res.status === 'cancelled' || (!res.active && res.status !== 'running'))) {
        stopPolling();
      }
    } catch (err) {
      loading = false;
    }
  }

  function startSSE() {
    const token = typeof localStorage !== 'undefined' ? localStorage.getItem('session_token') : '';
    const baseUrl = typeof window !== 'undefined' && window.location.port === '5173' ? 'http://localhost:3131' : '';

    try {
      eventSource = new EventSource(`${baseUrl}/api/batch/events?token=${token}`);

      eventSource.addEventListener('status', (e) => {
        try {
          const data = JSON.parse(e.data);
          status = data;
          loading = false;

          // Auto-scroll logs
          setTimeout(scrollLogsToBottom, 50);
        } catch {}
      });

      eventSource.addEventListener('log', (e) => {
        try {
          const log = JSON.parse(e.data);
          if (status && status.logs) {
            status = { ...status, logs: [...status.logs, log] };
          }
          setTimeout(scrollLogsToBottom, 50);
        } catch {}
      });

      eventSource.addEventListener('complete', () => {
        stopPolling();
        fetchStatus(); // Final fetch to get complete state
      });

      eventSource.onerror = () => {
        // SSE failed, fall back to polling
        if (eventSource) {
          eventSource.close();
          eventSource = null;
        }
        if (!pollInterval) {
          pollInterval = setInterval(fetchStatus, 1000);
        }
      };
    } catch {
      // SSE not supported, use polling
      pollInterval = setInterval(fetchStatus, 1000);
    }
  }

  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function scrollLogsToBottom() {
    const container = document.getElementById('log-container');
    if (container) {
      container.scrollTop = container.scrollHeight;
    }
  }

  async function cancelBatch() {
    try {
      await api.post('/api/batch/cancel');
      fetchStatus();
    } catch (err) {
      // ignore
    }
  }

  onMount(() => {
    fetchStatus();
    startSSE();
    // Also start polling as fallback
    pollInterval = setInterval(fetchStatus, 1500);
  });

  onDestroy(() => {
    stopPolling();
  });

  let progressPercent = $derived(
    status && status.total > 0 ? Math.round((status.current / status.total) * 100) : 0
  );

  let statusColor = $derived(
    status?.status === 'running' ? 'blue' :
    status?.status === 'completed' ? 'green' :
    status?.status === 'cancelled' ? 'orange' : 'gray'
  );

  let statusLabel = $derived(
    status?.status === 'running' ? 'Running' :
    status?.status === 'completed' ? 'Completed' :
    status?.status === 'cancelled' ? 'Cancelled' : 'Idle'
  );

  function formatTime(ts) {
    if (!ts) return '';
    const d = new Date(ts);
    return d.toLocaleTimeString('default', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function getLogColor(level) {
    if (level === 'error') return 'text-red-400';
    if (level === 'warn') return 'text-yellow-400 font-medium';
    if (level === 'success') return 'text-emerald-400';
    return 'text-text-base';
  }
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold mb-1">Add Accounts</h1>
      <p class="text-text-muted text-sm">Real-time batch account addition progress</p>
    </div>
    <button
      class="px-4 py-2 text-sm font-medium border border-border rounded-lg text-text-muted hover:text-text-base hover:bg-sidebar-hover transition-colors"
      onclick={onback}
    >
      Back to Accounts
    </button>
  </div>

  {#if loading}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 p-8 flex items-center justify-center min-h-[200px]">
      <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
    </div>
  {:else if status}
    <!-- Status Card -->
    <div class="rounded-xl border border-border bg-bg-sidebar/50 p-6 space-y-4">
      <!-- Title + Status + Cancel -->
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <h2 class="text-lg font-semibold">
            Adding Accounts ({status.current}/{status.total})
          </h2>
          <Badge text={statusLabel} color={statusColor} />
        </div>
        {#if status.active}
          <button
            class="px-3 py-1.5 text-sm text-red-400 hover:text-red-300 hover:bg-red-400/10 rounded-lg transition-colors"
            onclick={cancelBatch}
          >
            Cancel
          </button>
        {/if}
      </div>

      <!-- Progress bar -->
      <div>
        <div class="flex justify-between text-sm text-text-muted mb-1">
          <span>Progress</span>
          <span>{progressPercent}%</span>
        </div>
        <div class="h-3 w-full rounded-full bg-bg-base overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-500 ease-out {status.status === 'completed' ? 'bg-emerald-500' : 'bg-accent'}"
            style="width: {progressPercent}%"
          ></div>
        </div>
      </div>

      <!-- Stats -->
      <div class="flex items-center gap-6 text-sm">
        <div class="flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-emerald-400"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
          <span class="text-emerald-400 font-medium">{status.success}</span>
          <span class="text-text-muted">added</span>
        </div>
        <div class="flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-red-400"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
          <span class="text-red-400 font-medium">{status.failed}</span>
          <span class="text-text-muted">failed</span>
        </div>
        <div class="flex items-center gap-2 text-text-muted">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/></svg>
          <span>{status.total} total</span>
        </div>
      </div>
    </div>

    <!-- Logs -->
    <div class="rounded-xl border border-border bg-bg-sidebar/50 overflow-hidden">
      <div class="px-6 py-3 border-b border-border flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-text-muted"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
        <h3 class="text-sm font-semibold">Logs</h3>
        <span class="text-xs bg-bg-base border border-border rounded-full px-2 py-0.5 text-text-muted">{status.logs?.length || 0}</span>
      </div>
      <div class="max-h-[400px] overflow-y-auto p-4 space-y-1 font-mono text-xs" id="log-container">
        {#if status.logs && status.logs.length > 0}
          {#each status.logs as log}
            <div class="flex gap-3 py-0.5">
              <span class="text-text-muted/60 shrink-0">{formatTime(log.timestamp)}</span>
              <span class={getLogColor(log.level)}>{log.message}</span>
            </div>
          {/each}
        {:else}
          <div class="text-text-muted text-center py-4">Waiting for logs...</div>
        {/if}
      </div>
    </div>
  {:else}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 p-8 text-center text-text-muted">
      <p>No batch job running. Go to Accounts and click Add to start.</p>
    </div>
  {/if}
</div>
