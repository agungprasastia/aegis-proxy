<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from './api.js';
  import Badge from './components/Badge.svelte';
  import { toast } from './stores/toast.js';

  let logs = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let totalCount = $state(0);
  let currentPage = $state(1);
  let limit = $state(20);
  let autoRefresh = $state(false);
  let refreshInterval;

  async function fetchLogs() {
    try {
      if (logs.length === 0) loading = true;
      error = null;
      const res = await api.get(`/api/logs?page=${currentPage}&limit=${limit}`);
      logs = res?.logs || (Array.isArray(res) ? res : []);
      totalCount = res?.total || logs.length;
    } catch (err) {
      error = err.message || 'Failed to load logs';
    } finally {
      loading = false;
    }
  }

  async function clearLogs() {
    // Clear logs locally (backend doesn't have a delete endpoint for logs)
    logs = [];
    totalCount = 0;
    currentPage = 1;
    toast.info('Logs cleared from view');
  }

  function toggleAutoRefresh() {
    autoRefresh = !autoRefresh;
    if (autoRefresh) {
      refreshInterval = setInterval(fetchLogs, 5000);
    } else if (refreshInterval) {
      clearInterval(refreshInterval);
    }
  }

  onMount(fetchLogs);

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });

  function nextPage() {
    if (currentPage * limit < totalCount) {
      currentPage++;
      fetchLogs();
    }
  }

  function prevPage() {
    if (currentPage > 1) {
      currentPage--;
      fetchLogs();
    }
  }

  function formatTime(timestamp) {
    if (!timestamp) return '-';
    try {
      const date = new Date(timestamp);
      return date.toLocaleTimeString('default', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    } catch { return '-'; }
  }

  function getLatencyColor(ms) {
    if (!ms) return 'text-text-muted';
    if (ms < 500) return 'text-emerald-400';
    if (ms < 2000) return 'text-yellow-400';
    return 'text-red-400';
  }
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold mb-1">Logs</h1>
      <p class="text-text-muted text-sm">Monitor proxy requests and responses.</p>
    </div>
    <div class="flex items-center gap-3">
      <!-- Auto-refresh toggle -->
      <label class="flex items-center gap-2 cursor-pointer text-sm text-text-muted hover:text-text-base transition-colors">
        <div class="relative">
          <input type="checkbox" class="sr-only" checked={autoRefresh} onchange={toggleAutoRefresh} />
          <div class="block w-10 h-6 rounded-full transition-colors {autoRefresh ? 'bg-accent/30 border border-accent' : 'bg-bg-sidebar border border-border'}"></div>
          <div class="absolute left-1 top-1 w-4 h-4 rounded-full transition-transform {autoRefresh ? 'translate-x-4 bg-accent' : 'bg-text-muted'}"></div>
        </div>
        Auto-refresh
      </label>
      <button 
        class="px-4 py-2 bg-red-500/10 text-red-400 hover:bg-red-500/20 rounded-lg text-sm font-medium transition-colors min-h-[44px]"
        onclick={clearLogs}
      >
        Clear
      </button>
    </div>
  </div>

  <!-- Table -->
  <div class="rounded-xl border border-border bg-bg-sidebar/50 overflow-hidden">
    {#if loading}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
        <p>Loading logs...</p>
      </div>
    {:else if error && logs.length === 0}
      <div class="p-8 flex flex-col items-center justify-center text-red-400 space-y-4 min-h-[300px]">
        <p>{error}</p>
        <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-text-base hover:bg-sidebar-hover transition-colors" onclick={fetchLogs}>
          Retry
        </button>
      </div>
    {:else if logs.length === 0}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <p>No logs recorded yet.</p>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="border-b border-border text-xs uppercase text-text-muted">
            <tr>
              <th class="px-4 py-3 font-medium">TIME</th>
              <th class="px-4 py-3 font-medium">MODEL</th>
              <th class="px-4 py-3 font-medium">PROVIDER</th>
              <th class="px-4 py-3 font-medium">LATENCY</th>
              <th class="px-4 py-3 font-medium">STATUS</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each logs as log}
              <tr class="transition-colors hover:bg-accent/5">
                <td class="px-4 py-3 text-text-muted font-mono text-xs">
                  {formatTime(log.created_at || log.timestamp)}
                </td>
                <td class="px-4 py-3">
                  <span class="font-mono text-sm bg-bg-base px-2 py-0.5 rounded border border-border">{log.model || '-'}</span>
                </td>
                <td class="px-4 py-3 text-text-muted">
                  {log.provider || '-'}
                </td>
                <td class="px-4 py-3 font-mono {getLatencyColor(log.latency_ms || log.latency)}">
                  {log.latency_ms || log.latency ? `${log.latency_ms || log.latency}ms` : '-'}
                </td>
                <td class="px-4 py-3">
                  <Badge 
                    text={log.status || 'ok'} 
                    color={log.status === 'success' || log.status === 'ok' || log.status_code === 200 ? 'green' : 'red'} 
                  />
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div class="p-4 border-t border-border flex items-center justify-between">
        <div class="text-sm text-text-muted">
          {totalCount} total
        </div>
        <div class="flex items-center gap-2">
          <button 
            class="px-3 py-1.5 rounded bg-bg-sidebar border border-border text-text-muted hover:text-text-base hover:bg-sidebar-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm min-h-[36px]"
            disabled={currentPage === 1}
            onclick={prevPage}
            aria-label="Previous page"
          >
            Prev
          </button>
          <span class="text-sm font-medium px-2">Page {currentPage}</span>
          <button 
            class="px-3 py-1.5 rounded bg-bg-sidebar border border-border text-text-muted hover:text-text-base hover:bg-sidebar-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm min-h-[36px]"
            disabled={currentPage * limit >= totalCount}
            onclick={nextPage}
            aria-label="Next page"
          >
            Next
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
