<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import ConfirmModal from './components/ConfirmModal.svelte';
  import { toast } from './stores/toast.js';

  const providerTypes = [
    { id: 'openai', name: 'OpenAI', baseUrl: 'https://api.openai.com/v1', authHeader: 'Authorization', models: ['gpt-5.4', 'gpt-5.2', 'gpt-4.1'] },
    { id: 'anthropic', name: 'Anthropic', baseUrl: 'https://api.anthropic.com/v1', authHeader: 'x-api-key', models: ['claude-opus-4.6', 'claude-sonnet-4.5', 'claude-haiku-4.5'] },
    { id: 'gemini', name: 'Gemini', baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai', authHeader: 'Authorization', models: ['gemini-2.5-pro', 'gemini-2.5-flash'] },
    { id: 'openrouter', name: 'OpenRouter', baseUrl: 'https://openrouter.ai/api/v1', authHeader: 'Authorization', models: ['openrouter/auto'] },
    { id: 'deepseek', name: 'DeepSeek', baseUrl: 'https://api.deepseek.com/v1', authHeader: 'Authorization', models: ['deepseek-chat', 'deepseek-reasoner'] },
    { id: 'groq', name: 'Groq', baseUrl: 'https://api.groq.com/openai/v1', authHeader: 'Authorization', models: ['llama-3.3-70b-versatile', 'openai/gpt-oss-120b'] },
    { id: 'glm', name: 'GLM', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', authHeader: 'Authorization', models: ['glm-5', 'glm-4.5'] },
    { id: 'minimax', name: 'MiniMax', baseUrl: 'https://api.minimax.io/v1', authHeader: 'Authorization', models: ['minimax-m2.5'] },
    { id: 'mistral', name: 'Mistral', baseUrl: 'https://api.mistral.ai/v1', authHeader: 'Authorization', models: ['mistral-large-latest', 'codestral-latest'] },
    { id: 'xai', name: 'xAI', baseUrl: 'https://api.x.ai/v1', authHeader: 'Authorization', models: ['grok-4', 'grok-code-fast-1'] },
  ];

  let providers = $state([]);
  let provider = $state(providerTypes[0].id);
  let apiKey = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let testingProvider = $state(null);
  let deleting = $state(false);
  let error = $state(null);
  let providerToRemove = $state(null);
  let showRemoveConfirm = $state(false);

  const statusClasses = {
    connected: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30',
    error: 'bg-red-500/10 text-red-400 border-red-500/30',
    untested: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/30',
  };

  function providerMeta(id) {
    return providerTypes.find(item => item.id === id) || { id, name: id, baseUrl: '', authHeader: 'Authorization', models: [] };
  }

  function savedStatus() {
    if (typeof localStorage === 'undefined') return {};
    try {
      return JSON.parse(localStorage.getItem('aegis_provider_status') || '{}');
    } catch {
      return {};
    }
  }

  function setSavedStatus(providerId, status) {
    if (typeof localStorage === 'undefined') return;
    const statuses = savedStatus();
    statuses[providerId] = status;
    localStorage.setItem('aegis_provider_status', JSON.stringify(statuses));
  }

  function removeSavedStatus(providerId) {
    if (typeof localStorage === 'undefined') return;
    const statuses = savedStatus();
    delete statuses[providerId];
    localStorage.setItem('aegis_provider_status', JSON.stringify(statuses));
  }

  function normalizeProviders(items) {
    const statuses = savedStatus();
    return items.map(item => {
      const id = typeof item === 'string' ? item : item.provider;
      const meta = providerMeta(id);
      return {
        id,
        name: meta.name,
        baseUrl: meta.baseUrl,
        authHeader: meta.authHeader,
        models: meta.models,
        status: statuses[id] || 'untested',
        maskedKey: '••••••••••••••••',
      };
    });
  }

  async function fetchProviders() {
    try {
      loading = true;
      error = null;
      const res = await api.get('/api/providers/apikey');
      providers = normalizeProviders(res?.providers || []);
    } catch (err) {
      error = err.message || 'Failed to load providers';
    } finally {
      loading = false;
    }
  }

  async function saveProvider() {
    if (!provider || !apiKey.trim()) {
      toast.error('Provider and API key are required');
      return;
    }

    try {
      saving = true;
      await api.post('/api/providers/apikey', { provider, api_key: apiKey.trim() });
      apiKey = '';
      setSavedStatus(provider, 'untested');
      await fetchProviders();
      toast.success('Provider saved');
    } catch (err) {
      toast.error(err.message || 'Failed to save provider');
    } finally {
      saving = false;
    }
  }

  async function testProvider(item, key = '') {
    try {
      testingProvider = item.id;
      await api.post('/api/providers/apikey/test', {
        provider: item.id,
        base_url: item.baseUrl,
        auth_header: item.authHeader,
        api_key: key,
      });
      setSavedStatus(item.id, 'connected');
      providers = providers.map(providerItem => providerItem.id === item.id ? { ...providerItem, status: 'connected' } : providerItem);
      toast.success(`${item.name} connected`);
    } catch (err) {
      setSavedStatus(item.id, 'error');
      providers = providers.map(providerItem => providerItem.id === item.id ? { ...providerItem, status: 'error' } : providerItem);
      toast.error(err.message || `Failed to connect ${item.name}`);
    } finally {
      testingProvider = null;
    }
  }

  async function removeProvider() {
    if (!providerToRemove) return;

    try {
      deleting = true;
      await api.delete(`/api/providers/apikey/${providerToRemove.id}`);
      removeSavedStatus(providerToRemove.id);
      providers = providers.filter(item => item.id !== providerToRemove.id);
      toast.success('Provider removed');
      showRemoveConfirm = false;
      providerToRemove = null;
    } catch (err) {
      toast.error(err.message || 'Failed to remove provider');
    } finally {
      deleting = false;
    }
  }

  function confirmRemove(item) {
    providerToRemove = item;
    showRemoveConfirm = true;
  }

  let selectedProvider = $derived(providerMeta(provider));

  onMount(fetchProviders);
</script>

<div class="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold mb-1">API-Key Providers</h1>
      <p class="text-text-muted text-sm">Add upstream provider keys, test connectivity, and remove unused providers.</p>
    </div>
  </div>

  <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_360px] items-start">
    <div class="rounded-xl border border-border bg-bg-sidebar/50 overflow-hidden">
      {#if loading}
        <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
          <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
          <p>Loading providers...</p>
        </div>
      {:else if error}
        <div class="p-8 flex flex-col items-center justify-center text-red-400 space-y-4 min-h-[300px]">
          <p>{error}</p>
          <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-text-base hover:bg-sidebar-hover transition-colors" onclick={fetchProviders}>Retry</button>
        </div>
      {:else if providers.length === 0}
        <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-3 min-h-[300px] text-center">
          <div class="w-12 h-12 rounded-xl bg-accent/10 text-accent flex items-center justify-center">
            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 18v3c0 .6.4 1 1 1h4v-3h3v-3h2l1.4-1.4a6.5 6.5 0 1 0-4-4Z"/><circle cx="16.5" cy="7.5" r=".5"/></svg>
          </div>
          <div>
            <p class="text-text-base font-medium">No API-key providers configured</p>
            <p class="text-sm text-text-muted mt-1">Save a provider key to enable upstream routing.</p>
          </div>
        </div>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="border-b border-border text-xs uppercase text-text-muted">
              <tr>
                <th class="px-4 py-3 font-medium">Provider</th>
                <th class="px-4 py-3 font-medium">Status</th>
                <th class="px-4 py-3 font-medium">API Key</th>
                <th class="px-4 py-3 font-medium">Models</th>
                <th class="px-4 py-3 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              {#each providers as item}
                <tr class="transition-colors hover:bg-accent/5 align-top">
                  <td class="px-4 py-3">
                    <div class="font-medium text-text-base">{item.name}</div>
                    <div class="text-xs text-text-muted break-all">{item.baseUrl}</div>
                  </td>
                  <td class="px-4 py-3">
                    <span class="inline-flex items-center px-2 py-1 rounded-full border text-xs font-medium {statusClasses[item.status] || statusClasses.untested}">{item.status}</span>
                  </td>
                  <td class="px-4 py-3 font-mono text-text-muted">{item.maskedKey}</td>
                  <td class="px-4 py-3">
                    <div class="flex flex-wrap gap-1 max-w-xs">
                      {#each item.models as model}
                        <span class="px-2 py-1 rounded-md bg-bg-base border border-border text-xs font-mono text-text-muted">{model}</span>
                      {/each}
                    </div>
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex items-center justify-end gap-2">
                      <button class="px-3 py-1.5 text-xs font-medium rounded-lg border border-border text-text-muted hover:text-accent hover:bg-accent/10 transition-colors disabled:opacity-50" onclick={() => testProvider(item)} disabled={testingProvider === item.id}>
                        {testingProvider === item.id ? 'Testing...' : 'Test'}
                      </button>
                      <button class="px-3 py-1.5 text-xs font-medium rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors" onclick={() => confirmRemove(item)}>Remove</button>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>

    <form class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-6 space-y-4" onsubmit={(event) => { event.preventDefault(); saveProvider(); }}>
      <div>
        <h2 class="text-sm font-semibold text-text-muted uppercase tracking-wider mb-1">Add Provider</h2>
        <p class="text-sm text-text-muted">Key is sent to backend once and masked after save.</p>
      </div>

      <label class="block space-y-2">
        <span class="text-sm font-medium">Provider Type</span>
        <select bind:value={provider} class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors">
          {#each providerTypes as item}
            <option value={item.id}>{item.name}</option>
          {/each}
        </select>
      </label>

      <label class="block space-y-2">
        <span class="text-sm font-medium">API Key</span>
        <input type="password" bind:value={apiKey} autocomplete="off" placeholder="Paste provider API key" class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-sm placeholder:text-text-muted focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors" />
      </label>

      <div class="rounded-lg bg-bg-base border border-border p-3 space-y-2">
        <div class="text-xs uppercase tracking-wider text-text-muted">Configured Models</div>
        <div class="flex flex-wrap gap-1">
          {#each selectedProvider.models as model}
            <span class="px-2 py-1 rounded-md bg-bg-sidebar border border-border text-xs font-mono text-text-muted">{model}</span>
          {/each}
        </div>
      </div>

      <div class="flex flex-col sm:flex-row gap-2">
        <button type="submit" class="flex-1 px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50" disabled={saving || !apiKey.trim()}>{saving ? 'Saving...' : 'Save Provider'}</button>
        <button type="button" class="px-4 py-2 bg-bg-base border border-border text-text-base rounded-lg font-medium hover:bg-sidebar-hover transition-colors disabled:opacity-50" disabled={!apiKey.trim() || testingProvider === provider} onclick={() => testProvider(selectedProvider, apiKey.trim())}>{testingProvider === provider ? 'Testing...' : 'Test Key'}</button>
      </div>
    </form>
  </div>

  <ConfirmModal
    bind:isOpen={showRemoveConfirm}
    title="Remove Provider"
    message={`Remove ${providerToRemove?.name || 'this provider'}? Requests will no longer use this API key.`}
    confirmText="Remove"
    isDanger={true}
    loading={deleting}
    onConfirm={removeProvider}
    onCancel={() => { providerToRemove = null; showRemoveConfirm = false; }}
  />
</div>
