<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import Badge from './components/Badge.svelte';
  import Modal from './components/Modal.svelte';
  import { toast } from './stores/toast.js';

  let filters = $state([]);
  let loading = $state(true);
  let error = $state(null);
  
  let showAddModal = $state(false);
  let isAdding = $state(false);

  let newPattern = $state('');
  let newReplacement = $state('');
  let newMode = $state('regex');

  async function fetchFilters(showLoading = true) {
    try {
      if (showLoading) loading = true;
      error = null;
      const res = await api.get('/api/filters');
      filters = Array.isArray(res) ? res : (Array.isArray(res?.data) ? res.data : []);
    } catch (err) {
      error = err.message || 'Failed to load filters';
    } finally {
      loading = false;
    }
  }

  async function addFilter() {
    if (!newPattern.trim() || !newReplacement.trim()) {
      toast.error('Both pattern and replacement are required');
      return;
    }
    try {
      isAdding = true;
      await api.post('/api/filters', { pattern: newPattern, replacement: newReplacement, mode: newMode, active: true });
      toast.success('Filter added successfully');
      showAddModal = false;
      newPattern = '';
      newReplacement = '';
      fetchFilters();
    } catch (err) {
      toast.error(err.message || 'Failed to add filter');
    } finally {
      isAdding = false;
    }
  }

  async function toggleFilter(id, active) {
    try {
      await api.put(`/api/filters/${id}`, { active: !active });
      toast.success(active ? 'Filter disabled' : 'Filter enabled');
      // Update local state immediately for instant UI feedback
      filters = filters.map(f => f.id === id ? { ...f, active: !active } : f);
    } catch (err) {
      toast.error(err.message || 'Failed to update filter');
    }
  }

  async function deleteFilter(id) {
    try {
      await api.delete(`/api/filters/${id}`);
      toast.success('Filter deleted');
      fetchFilters();
    } catch (err) {
      toast.error(err.message || 'Failed to delete filter');
    }
  }

  onMount(fetchFilters);
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold mb-1">Filters</h1>
      <p class="text-text-muted text-sm">Content filter rules for request/response modification.</p>
    </div>
    <button 
      class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors flex items-center gap-2 min-h-[44px]"
      onclick={() => showAddModal = true}
    >
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
      Add Filter
    </button>
  </div>

  <!-- Table -->
  <div class="rounded-xl border border-border bg-bg-sidebar/50 overflow-hidden">
    {#if loading}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
        <p>Loading filters...</p>
      </div>
    {:else if error}
      <div class="p-8 flex flex-col items-center justify-center text-red-400 space-y-4 min-h-[300px]">
        <p>{error}</p>
        <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-text-base hover:bg-sidebar-hover transition-colors" onclick={fetchFilters}>Retry</button>
      </div>
    {:else if filters.length === 0}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <p>No filters created yet.</p>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="border-b border-border text-xs uppercase text-text-muted">
            <tr>
              <th class="px-4 py-3 font-medium w-16">STATUS</th>
              <th class="px-4 py-3 font-medium">MODE</th>
              <th class="px-4 py-3 font-medium">PATTERN</th>
              <th class="px-4 py-3 font-medium">REPLACEMENT</th>
              <th class="px-4 py-3 font-medium w-20">ACTIONS</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each filters as filter}
              <tr class="transition-colors hover:bg-accent/5">
                <td class="px-4 py-3">
                  <button 
                    class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors {filter.active ? 'bg-accent' : 'bg-bg-base border border-border'}"
                    role="switch" 
                    aria-checked={filter.active}
                    onclick={() => toggleFilter(filter.id, filter.active)}
                  >
                    <span class="pointer-events-none absolute left-0.5 inline-block h-4 w-4 transform rounded-full shadow transition duration-200 {filter.active ? 'translate-x-4 bg-white' : 'translate-x-0 bg-text-muted'}"></span>
                  </button>
                </td>
                <td class="px-4 py-3">
                  <Badge text={filter.mode || 'string'} color={filter.mode === 'regex' ? 'purple' : 'gray'} />
                </td>
                <td class="px-4 py-3 font-mono text-sm text-emerald-400 max-w-[200px] truncate" title={filter.pattern}>
                  {filter.pattern}
                </td>
                <td class="px-4 py-3 font-mono text-sm text-yellow-400 max-w-[200px] truncate" title={filter.replacement}>
                  {filter.replacement}
                </td>
                <td class="px-4 py-3">
                  <button 
                    class="p-1.5 text-text-muted hover:text-red-400 rounded hover:bg-red-400/10 transition-colors"
                    title="Delete filter"
                    onclick={() => deleteFilter(filter.id)}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <!-- Add Filter Modal -->
  <Modal bind:isOpen={showAddModal} title="Add Filter" onClose={() => showAddModal = false}>
    <div class="p-6 space-y-4">
      <div>
        <label for="filterMode" class="block text-sm font-medium text-text-muted mb-1">Mode</label>
        <select id="filterMode" bind:value={newMode} class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 focus:outline-none focus:border-accent transition-colors">
          <option value="regex">Regex</option>
          <option value="string">String</option>
        </select>
      </div>
      <div>
        <label for="pattern" class="block text-sm font-medium text-text-muted mb-1">Pattern</label>
        <input id="pattern" type="text" bind:value={newPattern} placeholder="Search pattern..." class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 font-mono text-sm placeholder:text-text-muted/50 focus:outline-none focus:border-accent transition-colors" />
      </div>
      <div>
        <label for="replacement" class="block text-sm font-medium text-text-muted mb-1">Replacement</label>
        <input id="replacement" type="text" bind:value={newReplacement} placeholder="Replace with..." class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 font-mono text-sm placeholder:text-text-muted/50 focus:outline-none focus:border-accent transition-colors" />
      </div>
      <div class="flex justify-end gap-3 pt-2">
        <button class="px-4 py-2 text-sm text-text-muted hover:text-text-base transition-colors" onclick={() => showAddModal = false}>Cancel</button>
        <button class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50" onclick={addFilter} disabled={isAdding || !newPattern.trim() || !newReplacement.trim()}>
          {isAdding ? 'Adding...' : 'Add Filter'}
        </button>
      </div>
    </div>
  </Modal>
</div>
