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

  let newProxyUrl = $state('');
  let newProxyType = $state('HTTP');

  async function fetchProxies() {
    try {
      loading = true;
      error = null;
      const res = await api.get('/api/proxies');
      proxies = Array.isArray(res) ? res : (Array.isArray(res?.data) ? res.data : []);
    } catch (err) {
      error = err.message || 'Failed to load proxies';
    } finally {
      loading = false;
    }
  }

  async function addProxy() {
    if (!newProxyUrl.trim()) {
      toast.error('Proxy URL is required');
      return;
    }
    try {
      isAdding = true;
      await api.post('/api/proxies', { url: newProxyUrl, type: newProxyType });
      toast.success('Proxy added successfully');
      showAddModal = false;
      newProxyUrl = '';
      fetchProxies();
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
      fetchProxies();
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
      fetchProxies();
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
      fetchProxies();
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
      fetchProxies();
    } catch (err) {
      toast.error(err.message || 'Failed to test proxies');
    } finally {
      isTesting = false;
    }
  }

  async function testProxy(id) {
    try {
      toast.info('Testing proxy...');
      // Use the global test endpoint (no per-proxy test route)
      await api.post('/api/proxies/test');
      toast.success('Proxy test completed');
      fetchProxies();
    } catch (err) {
      toast.error(err.message || 'Proxy test failed');
    }
  }

  onMount(fetchProxies);

  let totalCount = $derived(proxies.length);
  let activeCount = $derived(proxies.filter(p => p.status === 'ok' || p.status === 'active').length);
  let failedCount = $derived(totalCount - activeCount);

  function getLatencyColor(ms) {
    if (!ms || ms <= 0) return 'text-text-muted';
    if (ms < 500) return 'text-emerald-400';
    if (ms < 2000) return 'text-yellow-400';
    return 'text-red-400';
  }
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold mb-1">Proxies</h1>
      <p class="text-text-muted text-sm">Manage proxy servers for routing AI requests.</p>
    </div>
    <button 
      class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors flex items-center gap-2 min-h-[44px]"
      onclick={() => showAddModal = true}
    >
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
      Add Proxy
    </button>
  </div>

  <!-- Stats -->
  <div class="grid grid-cols-3 gap-4">
    <div class="rounded-xl border border-border bg-bg-sidebar p-4">
      <div class="text-2xl font-bold">{totalCount}</div>
      <div class="text-sm text-text-muted">Total</div>
    </div>
    <div class="rounded-xl border border-border bg-bg-sidebar p-4">
      <div class="text-2xl font-bold text-emerald-400">{activeCount}</div>
      <div class="text-sm text-text-muted">Active</div>
    </div>
    <div class="rounded-xl border border-border bg-bg-sidebar p-4">
      <div class="text-2xl font-bold text-red-400">{failedCount}</div>
      <div class="text-sm text-text-muted">Failed</div>
    </div>
  </div>

  <!-- Table -->
  <div class="rounded-xl border border-border bg-bg-sidebar/50 overflow-hidden">
    <!-- Action bar -->
    <div class="p-3 border-b border-border flex items-center justify-end gap-2">
      <button 
        class="px-3 py-1.5 bg-bg-base border border-border text-text-muted hover:text-text-base rounded-md text-xs font-medium transition-colors disabled:opacity-50 min-h-[36px]"
        disabled={isTesting || proxies.length === 0}
        onclick={testAll}
      >
        {#if isTesting}Testing...{:else}Test All{/if}
      </button>
      <button 
        class="px-3 py-1.5 bg-bg-base border border-border text-text-muted hover:text-red-400 rounded-md text-xs font-medium transition-colors disabled:opacity-50 min-h-[36px]"
        disabled={failedCount === 0}
        onclick={() => showConfirmDeleteFailed = true}
      >
        Delete Failed
      </button>
      <button 
        class="px-3 py-1.5 bg-bg-base border border-border text-text-muted hover:text-red-400 rounded-md text-xs font-medium transition-colors disabled:opacity-50 min-h-[36px]"
        disabled={proxies.length === 0}
        onclick={() => showConfirmDeleteAll = true}
      >
        Delete All
      </button>
    </div>

    {#if loading}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
        <p>Loading proxies...</p>
      </div>
    {:else if proxies.length === 0}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <p>No proxies configured yet.</p>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="border-b border-border text-xs uppercase text-text-muted">
            <tr>
              <th class="px-4 py-3 font-medium w-16">STATUS</th>
              <th class="px-4 py-3 font-medium">TYPE</th>
              <th class="px-4 py-3 font-medium">HOST/URL</th>
              <th class="px-4 py-3 font-medium">LATENCY</th>
              <th class="px-4 py-3 font-medium w-24">ACTIONS</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each proxies as proxy}
              <tr class="transition-colors hover:bg-accent/5">
                <td class="px-4 py-3">
                  <div class="w-2.5 h-2.5 rounded-full {proxy.status === 'ok' || proxy.status === 'active' ? 'bg-emerald-500' : 'bg-red-500'}"></div>
                </td>
                <td class="px-4 py-3">
                  <Badge text={proxy.type || 'HTTP'} color={proxy.type === 'SOCKS5' ? 'blue' : 'gray'} />
                </td>
                <td class="px-4 py-3 font-mono text-sm truncate max-w-[250px]" title={proxy.url || proxy.host}>
                  {proxy.url || proxy.host || '-'}
                </td>
                <td class="px-4 py-3 font-mono {getLatencyColor(proxy.latency_ms || proxy.latency)}">
                  {proxy.latency_ms || proxy.latency ? `${proxy.latency_ms || proxy.latency}ms` : '-'}
                </td>
                <td class="px-4 py-3">
                  <div class="flex items-center gap-1">
                    <button 
                      class="p-1.5 text-text-muted hover:text-accent rounded hover:bg-accent/10 transition-colors"
                      title="Test"
                      onclick={() => testProxy(proxy.id)}
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                    </button>
                    <button 
                      class="p-1.5 text-text-muted hover:text-red-400 rounded hover:bg-red-400/10 transition-colors"
                      title="Delete"
                      onclick={() => deleteProxy(proxy.url)}
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>
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

  <!-- Add Proxy Modal -->
  <Modal bind:isOpen={showAddModal} title="Add Proxy" onClose={() => showAddModal = false}>
    <div class="p-6 space-y-4">
      <div>
        <label for="proxyType" class="block text-sm font-medium text-text-muted mb-1">Type</label>
        <select id="proxyType" bind:value={newProxyType} class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 focus:outline-none focus:border-accent transition-colors">
          <option value="HTTP">HTTP/HTTPS</option>
          <option value="SOCKS5">SOCKS5</option>
        </select>
      </div>
      <div>
        <label for="proxyUrl" class="block text-sm font-medium text-text-muted mb-1">URL</label>
        <input id="proxyUrl" type="text" bind:value={newProxyUrl} placeholder="http://user:pass@host:port" class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 font-mono text-sm placeholder:text-text-muted/50 focus:outline-none focus:border-accent transition-colors" />
      </div>
      <div class="flex justify-end gap-3 pt-2">
        <button class="px-4 py-2 text-sm text-text-muted hover:text-text-base transition-colors" onclick={() => showAddModal = false}>Cancel</button>
        <button class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50" onclick={addProxy} disabled={isAdding || !newProxyUrl.trim()}>
          {isAdding ? 'Adding...' : 'Add Proxy'}
        </button>
      </div>
    </div>
  </Modal>

  <ConfirmModal bind:isOpen={showConfirmDeleteFailed} title="Delete Failed Proxies" message="Delete all proxies that failed their last test? This cannot be undone." confirmText="Delete Failed" isDanger={true} loading={isDeleting} onConfirm={deleteFailed} onCancel={() => showConfirmDeleteFailed = false} />
  <ConfirmModal bind:isOpen={showConfirmDeleteAll} title="Delete All Proxies" message="Delete ALL proxies? Your traffic will no longer be routed through them." confirmText="Delete All" isDanger={true} loading={isDeleting} onConfirm={deleteAll} onCancel={() => showConfirmDeleteAll = false} />
</div>
