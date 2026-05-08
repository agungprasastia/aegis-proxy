<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import Badge from './components/Badge.svelte';
  import Modal from './components/Modal.svelte';
  import ConfirmModal from './components/ConfirmModal.svelte';
  import { toast } from './stores/toast.js';

  let proxies = $state([]);
  let loading = $state(true);
  let error = $state(null);

  let showAddModal = $state(false);
  let showConfirmDeleteFailed = $state(false);
  let showConfirmDeleteAll = $state(false);

  let isAdding = $state(false);
  let isDeleting = $state(false);
  let isTesting = $state(false);
  let isSavingConfig = $state(false);

  let newProxyUrl = $state('');
  let newProxyType = $state('HTTP');
  let newProxyRegion = $state('');

  let proxyConfig = $state({
    for_kiro: true,
    for_codebuddy: true,
    for_wavespeed: false,
    for_codex: false,
    for_login: false,
    auto_test_enabled: false,
    auto_test_interval_min: 5,
    auto_delete_failed: false
  });

  const providerToggles = [
    { key: 'for_kiro', label: 'Kiro' },
    { key: 'for_codebuddy', label: 'CodeBuddy' },
    { key: 'for_wavespeed', label: 'Wavespeed' },
    { key: 'for_codex', label: 'Codex' },
    { key: 'for_login', label: 'Auto-Login' }
  ];

  async function fetchProxies() {
    try {
      loading = true;
      error = null;
      const [proxyRes, configRes] = await Promise.all([
        api.get('/api/proxies'),
        api.get('/api/proxies/config')
      ]);
      proxies = Array.isArray(proxyRes) ? proxyRes : (Array.isArray(proxyRes?.data) ? proxyRes.data : []);
      proxyConfig = {
        ...proxyConfig,
        ...(configRes || {})
      };
    } catch (err) {
      error = err.message || 'Failed to load proxies';
    } finally {
      loading = false;
    }
  }

  async function saveProxyConfig(patch) {
    const previous = { ...proxyConfig };
    proxyConfig = { ...proxyConfig, ...patch };

    try {
      isSavingConfig = true;
      const next = await api.put('/api/proxies/config', patch);
      proxyConfig = { ...proxyConfig, ...(next || {}) };
      if (Object.keys(patch).some((key) => key.startsWith('for_'))) {
        await fetchProxies();
      }
    } catch (err) {
      proxyConfig = previous;
      toast.error(err.message || 'Failed to save proxy settings');
    } finally {
      isSavingConfig = false;
    }
  }

  async function addProxy() {
    if (!newProxyUrl.trim()) {
      toast.error('Proxy URL is required');
      return;
    }

    try {
      isAdding = true;
      await api.post('/api/proxies', {
        url: newProxyUrl.trim(),
        type: newProxyType,
        region: newProxyRegion.trim().toUpperCase()
      });
      toast.success('Proxy added successfully');
      showAddModal = false;
      newProxyUrl = '';
      newProxyType = 'HTTP';
      newProxyRegion = '';
      await fetchProxies();
    } catch (err) {
      toast.error(err.message || 'Failed to add proxy');
    } finally {
      isAdding = false;
    }
  }

  async function deleteFailed() {
    try {
      isDeleting = true;
      await api.delete('/api/proxies/failed');
      toast.success('Failed proxies deleted');
      showConfirmDeleteFailed = false;
      await fetchProxies();
    } catch (err) {
      toast.error(err.message || 'Failed to delete proxies');
    } finally {
      isDeleting = false;
    }
  }

  async function deleteAll() {
    try {
      isDeleting = true;
      await api.delete('/api/proxies/all');
      toast.success('All proxies deleted');
      showConfirmDeleteAll = false;
      await fetchProxies();
    } catch (err) {
      toast.error(err.message || 'Failed to delete proxies');
    } finally {
      isDeleting = false;
    }
  }

  async function deleteProxy(proxyUrl) {
    try {
      await api.delete('/api/proxies', { url: proxyUrl });
      toast.success('Proxy deleted');
      await fetchProxies();
    } catch (err) {
      toast.error(err.message || 'Failed to delete proxy');
    }
  }

  async function testAll() {
    try {
      isTesting = true;
      toast.info('Testing proxies...');
      await api.post('/api/proxies/test');
      toast.success('Proxy testing completed');
      await fetchProxies();
    } catch (err) {
      toast.error(err.message || 'Failed to test proxies');
    } finally {
      isTesting = false;
    }
  }

  async function testProxy() {
    await testAll();
  }

  function formatStatus(proxy) {
    return proxy.status === 'ok' || proxy.status === 'active' ? 'ok' : 'failed';
  }

  function formatLatency(proxy) {
    const latency = proxy.latency_ms || proxy.latency;
    return latency ? `${latency}ms` : '—';
  }

  function getLatencyColor(ms) {
    if (!ms || ms <= 0) return 'text-text-muted';
    if (ms < 1500) return 'text-emerald-400';
    if (ms < 3000) return 'text-yellow-400';
    return 'text-red-400';
  }

  function formatLastChecked(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit', second: '2-digit' });
  }

  onMount(fetchProxies);

  let totalCount = $derived(proxies.length);
  let activeCount = $derived(proxies.filter((p) => formatStatus(p) === 'ok').length);
  let failedCount = $derived(totalCount - activeCount);
</script>

<div class="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500 max-w-6xl mx-auto">
  <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
    <div>
      <h1 class="text-3xl font-bold text-white mb-1">Proxy</h1>
      <p class="text-sm text-text-muted">Route provider requests through proxies</p>
    </div>
    <button
      class="inline-flex items-center justify-center rounded-full bg-[#1f8fff] px-5 py-3 text-sm font-semibold text-white transition hover:bg-[#3a9cff] disabled:opacity-50"
      onclick={() => showAddModal = true}
    >
      Add Proxy
    </button>
  </div>

  <a
    href="https://www.webshare.io/"
    target="_blank"
    rel="noreferrer"
    class="flex items-center justify-between gap-3 rounded-2xl border border-[#12345b] bg-[#091a30] px-4 py-3 text-sm text-[#b7d7ff] transition hover:border-[#1f8fff]/60 hover:bg-[#0b1f38]"
  >
    <div class="flex items-center gap-3">
      <div class="flex h-7 w-7 items-center justify-center rounded-full bg-[#12345b] text-[#69b3ff]">🛡</div>
      <div>
        <span class="font-semibold text-white">Need reliable proxies?</span>
        <span class="text-text-muted"> Get affordable residential &amp; datacenter proxies from Webshare — supports HTTP, HTTPS, SOCKS5.</span>
      </div>
    </div>
    <span class="text-lg">↗</span>
  </a>

  <div class="rounded-3xl border border-border bg-[#111114] p-5 shadow-[0_18px_60px_rgba(0,0,0,0.35)]">
    <div class="flex flex-col gap-5">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-center">
            <div class="text-xs font-semibold uppercase tracking-[0.18em] text-text-muted">Apply to</div>
            <div class="flex flex-wrap gap-x-5 gap-y-3 text-sm text-white">
              {#each providerToggles as toggle}
                <label class="inline-flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={proxyConfig[toggle.key]}
                    disabled={isSavingConfig}
                    onchange={(e) => saveProxyConfig({ [toggle.key]: e.currentTarget.checked })}
                    class="h-4 w-4 rounded border-border bg-bg-base text-accent focus:ring-accent"
                  />
                  <span>{toggle.label}</span>
                </label>
              {/each}
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-x-6 gap-y-3 text-sm text-white">
            <label class="inline-flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={proxyConfig.auto_test_enabled}
                disabled={isSavingConfig}
                onchange={(e) => saveProxyConfig({ auto_test_enabled: e.currentTarget.checked })}
                class="h-4 w-4 rounded border-border bg-bg-base text-accent focus:ring-accent"
              />
              <span>Auto-test every</span>
            </label>

            <select
              class="rounded-lg border border-border bg-bg-base px-3 py-1.5 text-sm text-white disabled:opacity-50"
              value={proxyConfig.auto_test_interval_min}
              disabled={!proxyConfig.auto_test_enabled || isSavingConfig}
              onchange={(e) => saveProxyConfig({ auto_test_interval_min: Number(e.currentTarget.value) })}
            >
              <option value={1}>1 min</option>
              <option value={5}>5 min</option>
              <option value={10}>10 min</option>
              <option value={15}>15 min</option>
              <option value={30}>30 min</option>
            </select>

            <label class="inline-flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={proxyConfig.auto_delete_failed}
                disabled={isSavingConfig}
                onchange={(e) => saveProxyConfig({ auto_delete_failed: e.currentTarget.checked })}
                class="h-4 w-4 rounded border-border bg-bg-base text-accent focus:ring-accent"
              />
              <span>Auto-delete failed</span>
            </label>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-3 text-xs sm:text-sm">
          <button
            class="text-text-muted transition hover:text-white disabled:opacity-50"
            disabled={failedCount === 0 || isDeleting}
            onclick={() => showConfirmDeleteFailed = true}
          >
            Delete Failed
          </button>
          <button
            class="text-text-muted transition hover:text-white disabled:opacity-50"
            disabled={totalCount === 0 || isDeleting}
            onclick={() => showConfirmDeleteAll = true}
          >
            Delete All
          </button>
          <button
            class="text-text-muted transition hover:text-white disabled:opacity-50"
            disabled={totalCount === 0 || isTesting}
            onclick={testAll}
          >
            {isTesting ? 'Testing…' : 'Test All'}
          </button>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-4 text-sm">
        <span class="text-text-muted">{totalCount} total</span>
        <span class="font-semibold text-emerald-400">{activeCount} ok</span>
        <span class="font-semibold text-red-400">{failedCount} failed</span>
      </div>
    </div>
  </div>

  <div class="overflow-hidden rounded-3xl border border-border bg-[#111114] shadow-[0_18px_60px_rgba(0,0,0,0.35)]">
    {#if loading}
      <div class="flex min-h-[320px] flex-col items-center justify-center gap-4 text-text-muted">
        <div class="h-8 w-8 rounded-full border-2 border-accent border-t-transparent animate-spin"></div>
        <p>Loading proxies...</p>
      </div>
    {:else if error}
      <div class="flex min-h-[320px] flex-col items-center justify-center gap-4 p-8 text-red-400">
        <p>{error}</p>
        <button class="rounded-lg border border-border px-4 py-2 text-text-base transition hover:bg-bg-base" onclick={fetchProxies}>Retry</button>
      </div>
    {:else if proxies.length === 0}
      <div class="flex min-h-[320px] flex-col items-center justify-center gap-4 p-8 text-text-muted">
        <p>No proxies configured yet.</p>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full min-w-[980px] text-left text-sm">
          <thead class="border-b border-border text-xs uppercase tracking-[0.14em] text-text-muted">
            <tr>
              <th class="px-5 py-4 font-medium">Status</th>
              <th class="px-4 py-4 font-medium">Type</th>
              <th class="px-4 py-4 font-medium">Region</th>
              <th class="px-4 py-4 font-medium">Host</th>
              <th class="px-4 py-4 font-medium">Port</th>
              <th class="px-4 py-4 font-medium">Latency</th>
              <th class="px-4 py-4 font-medium">Last Checked</th>
              <th class="px-4 py-4 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border/80">
            {#each proxies as proxy}
              <tr class="transition hover:bg-white/[0.02]">
                <td class="px-5 py-4">
                  <div class="flex items-center gap-2">
                    <span class={`inline-block h-2.5 w-2.5 rounded-full ${formatStatus(proxy) === 'ok' ? 'bg-emerald-500' : 'bg-red-500'}`}></span>
                    <span class={`font-medium ${formatStatus(proxy) === 'ok' ? 'text-emerald-400' : 'text-red-400'}`}>{formatStatus(proxy)}</span>
                  </div>
                </td>
                <td class="px-4 py-4">
                  <Badge text={(proxy.type || 'HTTP').toUpperCase()} color="gray" />
                </td>
                <td class="px-4 py-4 text-text-base">{proxy.region || '—'}</td>
                <td class="px-4 py-4 font-mono text-white">{proxy.host || proxy.url || '—'}</td>
                <td class="px-4 py-4 font-mono text-text-base">{proxy.port || '—'}</td>
                <td class={`px-4 py-4 font-mono ${getLatencyColor(proxy.latency_ms || proxy.latency)}`}>{formatLatency(proxy)}</td>
                <td class="px-4 py-4 text-text-base">{formatLastChecked(proxy.last_checked)}</td>
                <td class="px-4 py-4">
                  <div class="flex items-center gap-2 text-text-muted">
                    <button class="rounded p-1.5 transition hover:bg-accent/10 hover:text-accent" title="Test proxy" onclick={() => testProxy(proxy.id)}>
                      ⚡
                    </button>
                    <button class="rounded p-1.5 transition hover:bg-red-500/10 hover:text-red-400" title="Delete proxy" onclick={() => deleteProxy(proxy.url)}>
                      🗑
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <Modal bind:isOpen={showAddModal} title="Add Proxy" onClose={() => showAddModal = false}>
    <div class="p-6 space-y-4">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label for="proxyType" class="mb-1 block text-sm font-medium text-text-muted">Type</label>
          <select id="proxyType" bind:value={newProxyType} class="w-full rounded-lg border border-border bg-bg-base px-3 py-2 text-white focus:border-accent focus:outline-none">
            <option value="HTTP">HTTP/HTTPS</option>
            <option value="SOCKS5">SOCKS5</option>
          </select>
        </div>
        <div>
          <label for="proxyRegion" class="mb-1 block text-sm font-medium text-text-muted">Region</label>
          <input id="proxyRegion" type="text" bind:value={newProxyRegion} placeholder="VN" class="w-full rounded-lg border border-border bg-bg-base px-3 py-2 text-white placeholder:text-text-muted/50 focus:border-accent focus:outline-none" />
        </div>
      </div>

      <div>
        <label for="proxyUrl" class="mb-1 block text-sm font-medium text-text-muted">URL</label>
        <input id="proxyUrl" type="text" bind:value={newProxyUrl} placeholder="http://user:pass@host:port" class="w-full rounded-lg border border-border bg-bg-base px-3 py-2 font-mono text-sm text-white placeholder:text-text-muted/50 focus:border-accent focus:outline-none" />
      </div>

      <div class="flex justify-end gap-3 pt-2">
        <button class="px-4 py-2 text-sm text-text-muted transition hover:text-text-base" onclick={() => showAddModal = false}>Cancel</button>
        <button class="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-white transition hover:bg-accent-hover disabled:opacity-50" onclick={addProxy} disabled={isAdding || !newProxyUrl.trim()}>
          {isAdding ? 'Adding...' : 'Add Proxy'}
        </button>
      </div>
    </div>
  </Modal>

  <ConfirmModal bind:isOpen={showConfirmDeleteFailed} title="Delete Failed Proxies" message="Delete all proxies that failed their last test? This cannot be undone." confirmText="Delete Failed" isDanger={true} loading={isDeleting} onConfirm={deleteFailed} onCancel={() => showConfirmDeleteFailed = false} />
  <ConfirmModal bind:isOpen={showConfirmDeleteAll} title="Delete All Proxies" message="Delete all proxies? Your traffic will no longer be routed through them." confirmText="Delete All" isDanger={true} loading={isDeleting} onConfirm={deleteAll} onCancel={() => showConfirmDeleteAll = false} />
</div>
