<script>
  import { onDestroy, onMount } from 'svelte';
  import { api } from './api.js';

  const providers = ['kiro', 'codebuddy', 'openai', 'anthropic', 'gemini', 'openrouter', 'deepseek', 'groq', 'glm', 'minimax', 'mistral', 'xai'];

  let rows = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let now = $state(Date.now());
  let tick;

  async function fetchQuota() {
    try {
      loading = true;
      error = null;
      const results = await Promise.all(providers.map(async (provider) => {
        try {
          const quotas = await api.get(`/api/quota/${provider}`);
          return (Array.isArray(quotas) ? quotas : []).map(item => ({ ...item, provider }));
        } catch {
          return [{ provider, account_id: 'untracked', used_tokens: 0, total_tokens: 0, remaining_tokens: 0, status: 'unknown', stale: true }];
        }
      }));
      rows = results.flat();
    } catch (err) {
      error = err.message || 'Failed to load quota data';
    } finally {
      loading = false;
    }
  }

  function usagePercent(row) {
    if (!row.total_tokens) return 0;
    return Math.min(100, Math.round((row.used_tokens / row.total_tokens) * 100));
  }

  function sourceLabel(row) {
    if (row.status === 'known' || row.status === 'exhausted') return 'Provider-reported';
    return row.stale ? 'Unknown / stale' : 'Estimated';
  }

  function badgeClass(row) {
    if (row.status === 'exhausted') return 'border-red-500/30 bg-red-500/10 text-red-400';
    if (row.status === 'known') return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400';
    return 'border-yellow-500/30 bg-yellow-500/10 text-yellow-400';
  }

  function countdown(row) {
    if (!row.reset_at) return 'No reset data';
    const resetAt = new Date(row.reset_at).getTime();
    const seconds = Math.max(0, Math.floor((resetAt - now) / 1000));
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const rest = seconds % 60;
    if (hours > 0) return `${hours}h ${minutes}m`;
    if (minutes > 0) return `${minutes}m ${rest}s`;
    return `${rest}s`;
  }

  onMount(() => {
    fetchQuota();
    tick = setInterval(() => now = Date.now(), 1000);
  });

  onDestroy(() => {
    if (tick) clearInterval(tick);
  });
</script>

<div class="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold mb-1">Quota</h1>
      <p class="text-text-muted text-sm">Advisory provider usage, remaining tokens, and reset countdowns.</p>
    </div>
    <button class="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors" onclick={fetchQuota} disabled={loading}>{loading ? 'Refreshing...' : 'Refresh'}</button>
  </div>

  <div class="rounded-xl border border-yellow-500/20 bg-yellow-500/10 p-4 text-sm text-yellow-100">
    Quota values are operational estimates unless labeled provider-reported. Use upstream dashboards for billing truth.
  </div>

  {#if loading && rows.length === 0}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 p-8 flex flex-col items-center justify-center min-h-[300px] text-text-muted space-y-4">
      <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
      <p>Loading quota...</p>
    </div>
  {:else if error}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 p-8 text-center text-red-400">
      <p>{error}</p>
    </div>
  {:else}
    <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {#each rows as row}
        <article class="rounded-xl border border-border bg-bg-sidebar/50 p-5 space-y-4">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h2 class="font-semibold capitalize">{row.provider}</h2>
              <p class="text-xs text-text-muted font-mono break-all">{row.account_id}</p>
            </div>
            <span class={`inline-flex items-center px-2 py-1 rounded-full border text-xs font-medium ${badgeClass(row)}`}>{sourceLabel(row)}</span>
          </div>

          <div class="space-y-2">
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">Used</span>
              <span>{row.used_tokens?.toLocaleString?.() || 0}{row.total_tokens ? ` / ${row.total_tokens.toLocaleString()}` : ''}</span>
            </div>
            <div class="h-2 rounded-full bg-bg-base overflow-hidden border border-border">
              <div class="h-full bg-accent transition-all" style={`width: ${usagePercent(row)}%`}></div>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3 text-sm">
            <div class="rounded-lg bg-bg-base border border-border p-3">
              <div class="text-xs uppercase tracking-wider text-text-muted mb-1">Remaining</div>
              <div class="font-mono">{row.total_tokens ? (row.remaining_tokens || 0).toLocaleString() : 'Unknown'}</div>
            </div>
            <div class="rounded-lg bg-bg-base border border-border p-3">
              <div class="text-xs uppercase tracking-wider text-text-muted mb-1">Reset</div>
              <div class="font-mono">{countdown(row)}</div>
            </div>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</div>
