<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import ConfirmModal from './components/ConfirmModal.svelte';
  import { toast } from './stores/toast.js';

  let apiKeyMasked = $state('');
  let apiKeyFull = $state('');
  let baseUrl = $state('http://localhost:3130/v1');
  let loading = $state(true);
  let error = $state(null);
  let showKey = $state(false);
  let isRegenerating = $state(false);
  let showConfirmRegenerate = $state(false);

  async function fetchApiKey() {
    try {
      loading = true;
      error = null;
      const res = await api.get('/api/apikey');
      apiKeyMasked = res?.api_key || '';
      apiKeyFull = res?.api_key_full || res?.api_key || '';
      if (res?.base_url || res?.baseUrl) baseUrl = res.base_url || res.baseUrl;
    } catch (err) {
      error = err.message || 'Failed to load API key';
    } finally {
      loading = false;
    }
  }

  async function regenerateApiKey() {
    try {
      isRegenerating = true;
      const res = await api.post('/api/apikey/regen', {});
      apiKeyFull = res?.api_key_full || res?.api_key || '';
      apiKeyMasked = maskKey(apiKeyFull);
      toast.success('API Key regenerated successfully');
      showConfirmRegenerate = false;
    } catch (err) {
      toast.error(err.message || 'Failed to regenerate API key');
    } finally {
      isRegenerating = false;
    }
  }

  onMount(fetchApiKey);

  function copyToClipboard(text, label) {
    navigator.clipboard.writeText(text).then(() => {
      toast.success(`${label} copied to clipboard`);
    }).catch(() => {
      toast.error(`Failed to copy ${label}`);
    });
  }

  function maskKey(key) {
    if (!key || key.length <= 8) return key || '';
    return `${key.substring(0, 4)}${'*'.repeat(Math.max(0, key.length - 8))}${key.substring(key.length - 4)}`;
  }

  let displayKey = $derived(showKey ? apiKeyFull : (apiKeyMasked || maskKey(apiKeyFull)));
</script>

<div class="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500 max-w-4xl mx-auto">
  <div>
    <h1 class="text-2xl font-bold text-white mb-1">API Key</h1>
    <p class="text-text-muted text-sm">Manage your Aegis Proxy API authentication credentials.</p>
  </div>

  {#if loading}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-8 flex flex-col items-center justify-center text-text-muted space-y-4">
      <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
      <p>Loading credentials...</p>
    </div>
  {:else if error}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-8 flex flex-col items-center justify-center text-red-400 space-y-4">
      <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="opacity-50"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>
      <p>{error}</p>
      <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-text-base hover:bg-border transition-colors" onclick={fetchApiKey}>
        Retry
      </button>
    </div>
  {:else}
    <div class="grid gap-6">
      <!-- API Key Section -->
      <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-6 relative overflow-hidden group">
        <div class="absolute top-0 right-0 w-32 h-32 bg-accent/5 rounded-full blur-3xl -mr-10 -mt-10 pointer-events-none"></div>
        
        <h2 class="text-sm font-semibold text-text-muted uppercase tracking-wider mb-4">Your API Key</h2>
        
        <div class="flex items-center gap-3 bg-bg-base border border-border rounded-lg p-3 relative z-10">
          <div class="flex-1 font-mono text-lg text-white tracking-widest break-all">
            {displayKey}
          </div>
          
          <button 
            class="p-2 text-text-muted hover:text-white rounded-md hover:bg-sidebar-hover transition-colors"
            onclick={() => showKey = !showKey}
            aria-label={showKey ? "Hide API key" : "Show API key"}
            title={showKey ? "Hide" : "Show"}
          >
            {#if showKey}
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/><path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/><line x1="2" x2="22" y1="2" y2="22"/></svg>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg>
            {/if}
          </button>
          
          <div class="w-px h-6 bg-border mx-1"></div>
          
          <button 
            class="p-2 text-text-muted hover:text-accent rounded-md hover:bg-accent/10 transition-colors"
            onclick={() => copyToClipboard(apiKeyFull, 'API Key')}
            title="Copy API Key"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
          </button>
        </div>
      </div>

      <!-- Base URL Section -->
      <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-6">
        <h2 class="text-sm font-semibold text-text-muted uppercase tracking-wider mb-4">Base URL</h2>
        <p class="text-sm text-text-muted mb-3">Use this URL as your OpenAI-compatible endpoint in client applications.</p>
        
        <div class="flex items-center gap-3 bg-bg-base border border-border rounded-lg p-3">
          <div class="flex-1 font-mono text-white break-all">
            {baseUrl}
          </div>
          <button 
            class="p-2 text-text-muted hover:text-accent rounded-md hover:bg-accent/10 transition-colors"
            onclick={() => copyToClipboard(baseUrl, 'Base URL')}
            title="Copy Base URL"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
          </button>
        </div>
      </div>

      <!-- Danger Zone -->
      <div class="rounded-xl border border-red-500/20 bg-red-500/5 p-6">
        <h2 class="text-sm font-semibold text-red-400 uppercase tracking-wider mb-2">Danger Zone</h2>
        <div class="flex flex-col sm:flex-row gap-4 items-start sm:items-center justify-between">
          <p class="text-sm text-text-muted">
            Regenerating your API key will immediately invalidate the old one. 
            All applications using the old key will stop working.
          </p>
          <button 
            class="whitespace-nowrap px-4 py-2 bg-red-500 hover:bg-red-600 text-white rounded-lg font-medium transition-colors shadow-[0_0_15px_rgba(239,68,68,0.3)]"
            onclick={() => showConfirmRegenerate = true}
          >
            Regenerate Key
          </button>
        </div>
      </div>
    </div>
  {/if}

  <ConfirmModal
    bind:isOpen={showConfirmRegenerate}
    title="Regenerate API Key"
    message="Are you absolutely sure? This will invalidate your current API key immediately. Any applications currently using it will need to be updated."
    confirmText="Yes, Regenerate"
    isDanger={true}
    loading={isRegenerating}
    onConfirm={regenerateApiKey}
    onCancel={() => showConfirmRegenerate = false}
  />
</div>