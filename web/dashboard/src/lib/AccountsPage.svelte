<script>
  import AddAccountModal from './AddAccountModal.svelte';

  let accounts = $state([]);
  let loading = $state(true);
  let actionLoading = $state('');
  let showModal = $state(false);
  let selectedProvider = $state('');

  const providers = [
    { name: 'Kiro', color: '#8B5CF6', locked: false },
    { name: 'CodeBuddy', color: '#3B82F6', locked: false },
    { name: 'Wavespeed', color: '#EAB308', locked: false },
    { name: 'Canva', color: '#06B6D4', locked: false },
    { name: 'Codex', color: '#6B7280', locked: true },
    { name: 'YepAPI', color: '#6B7280', locked: true }
  ];

  let accountsByProvider = $derived(groupByProvider(accounts));

  function groupByProvider(accs) {
    const grouped = {};
    for (const p of providers) {
      const key = p.name.toLowerCase();
      grouped[key] = { total: 0, active: 0, exhausted: 0, banned: 0, error: 0 };
    }
    for (const acc of accs) {
      const key = (acc.provider || '').toLowerCase();
      if (!grouped[key]) {
        grouped[key] = { total: 0, active: 0, exhausted: 0, banned: 0, error: 0 };
      }
      grouped[key].total++;
      const status = (acc.status || '').toLowerCase();
      if (status === 'active') grouped[key].active++;
      else if (status === 'exhausted') grouped[key].exhausted++;
      else if (status === 'banned') grouped[key].banned++;
      else if (status === 'error') grouped[key].error++;
    }
    return grouped;
  }

  function getHeaders() {
    return {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('session_token') || ''}`
    };
  }

  async function fetchAccounts() {
    loading = true;
    try {
      const res = await fetch('/api/accounts', { headers: getHeaders() });
      if (res.ok) {
        const data = await res.json();
        accounts = Array.isArray(data) ? data : [];
      } else {
        accounts = [];
      }
    } catch {
      accounts = [];
    } finally {
      loading = false;
    }
  }

  async function syncAccounts() {
    actionLoading = 'sync';
    try {
      await fetch('/api/accounts/sync', { method: 'POST', headers: getHeaders() });
      await fetchAccounts();
    } catch { /* ignore */ }
    finally { actionLoading = ''; }
  }

  async function deleteInactive() {
    if (!confirm('Delete all inactive accounts? This cannot be undone.')) return;
    actionLoading = 'inactive';
    try {
      await fetch('/api/accounts/inactive', { method: 'DELETE', headers: getHeaders() });
      await fetchAccounts();
    } catch { /* ignore */ }
    finally { actionLoading = ''; }
  }

  async function deleteAll() {
    if (!confirm('Delete ALL accounts? This action is irreversible!')) return;
    actionLoading = 'all';
    try {
      await fetch('/api/accounts/all', { method: 'DELETE', headers: getHeaders() });
      await fetchAccounts();
    } catch { /* ignore */ }
    finally { actionLoading = ''; }
  }

  function openAddModal(providerName) {
    selectedProvider = providerName.toLowerCase();
    showModal = true;
  }

  function handleModalClose() {
    fetchAccounts();
  }

  $effect(() => {
    fetchAccounts();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
    <div>
      <h2 class="text-2xl font-bold text-white">Accounts</h2>
      <p class="text-text-muted text-sm mt-1">Manage accounts for the AI proxy</p>
    </div>
    <div class="flex items-center gap-2 flex-wrap">
      <button
        class="px-3 py-2 text-sm font-medium rounded-lg border border-border text-text-muted hover:text-white hover:bg-sidebar-hover transition-colors flex items-center gap-2 disabled:opacity-50"
        onclick={syncAccounts}
        disabled={actionLoading !== ''}
      >
        {#if actionLoading === 'sync'}
          <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
        {:else}
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.2"/></svg>
        {/if}
        Sync Accounts
      </button>
      <button
        class="px-3 py-2 text-sm font-medium rounded-lg border border-border text-text-muted hover:text-orange-400 hover:border-orange-500/30 hover:bg-orange-500/10 transition-colors flex items-center gap-2 disabled:opacity-50"
        onclick={deleteInactive}
        disabled={actionLoading !== ''}
      >
        {#if actionLoading === 'inactive'}
          <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
        {:else}
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>
        {/if}
        Delete Inactive
      </button>
      <button
        class="px-3 py-2 text-sm font-medium rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors flex items-center gap-2 disabled:opacity-50"
        onclick={deleteAll}
        disabled={actionLoading !== ''}
      >
        {#if actionLoading === 'all'}
          <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
        {:else}
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/></svg>
        {/if}
        Delete All
      </button>
    </div>
  </div>

  <!-- Loading State -->
  {#if loading}
    <div class="flex items-center justify-center py-20">
      <svg class="animate-spin h-8 w-8 text-accent" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
    </div>
  {:else}
    <!-- Provider Cards Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each providers as provider}
        {@const key = provider.name.toLowerCase()}
        {@const stats = accountsByProvider[key] || { total: 0, active: 0, exhausted: 0, banned: 0, error: 0 }}
        <div class="rounded-xl border border-border bg-[#161b22] p-5 flex flex-col gap-4 transition-colors hover:border-[#30363d]">
          <!-- Card Header -->
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div
                class="w-10 h-10 rounded-full flex items-center justify-center text-white font-bold text-sm shrink-0"
                style="background-color: {provider.color}"
              >
                {provider.name.charAt(0)}
              </div>
              <div>
                <h3 class="text-white font-semibold text-sm">{provider.name}</h3>
                <p class="text-text-muted text-xs">{stats.total} account{stats.total !== 1 ? 's' : ''}</p>
              </div>
            </div>
            {#if provider.locked}
              <button
                class="px-3 py-1.5 text-xs font-medium rounded-lg bg-[#30363d] text-text-muted cursor-not-allowed flex items-center gap-1.5"
                disabled
              >
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
                Locked
              </button>
            {:else}
              <button
                class="px-3 py-1.5 text-xs font-medium rounded-lg text-white transition-colors hover:opacity-90"
                style="background-color: {provider.color}"
                onclick={() => openAddModal(provider.name)}
              >
                + Add
              </button>
            {/if}
          </div>

          <!-- Status Grid -->
          <div class="grid grid-cols-2 gap-2">
            <div class="rounded-lg bg-[#0d1117] px-3 py-2 flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-[#22c55e] shrink-0"></div>
              <span class="text-xs text-text-muted">Active</span>
              <span class="text-xs text-white font-medium ml-auto">{stats.active}</span>
            </div>
            <div class="rounded-lg bg-[#0d1117] px-3 py-2 flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-[#f59e0b] shrink-0"></div>
              <span class="text-xs text-text-muted">Exhausted</span>
              <span class="text-xs text-white font-medium ml-auto">{stats.exhausted}</span>
            </div>
            <div class="rounded-lg bg-[#0d1117] px-3 py-2 flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-[#ef4444] shrink-0"></div>
              <span class="text-xs text-text-muted">Banned</span>
              <span class="text-xs text-white font-medium ml-auto">{stats.banned}</span>
            </div>
            <div class="rounded-lg bg-[#0d1117] px-3 py-2 flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-[#dc2626] shrink-0"></div>
              <span class="text-xs text-text-muted">Error</span>
              <span class="text-xs text-white font-medium ml-auto">{stats.error}</span>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Add Account Modal -->
<AddAccountModal bind:show={showModal} bind:provider={selectedProvider} onclose={handleModalClose} />
