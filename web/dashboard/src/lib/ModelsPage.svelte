<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import Badge from './components/Badge.svelte';

  let models = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let searchQuery = $state('');
  let selectedFilter = $state('All');

  const filters = ['All', 'Standard', 'MAX', 'Wavespeed', 'Codex'];

  const tierColors = {
    'Standard': 'green',
    'standard': 'green',
    'Max': 'purple',
    'max': 'purple',
    'MAX': 'purple',
    'Wavespeed': 'blue',
    'wavespeed': 'blue',
    'Codex': 'orange',
    'codex': 'orange'
  };

  async function fetchModels() {
    try {
      loading = true;
      error = null;
      const res = await api.get('/api/models');
      // Handle different response shapes
      if (Array.isArray(res)) {
        models = res;
      } else if (res?.data && Array.isArray(res.data)) {
        models = res.data;
      } else if (typeof res === 'object' && res !== null) {
        // Response is {standard: [...], max: [...], ...}
        // Each model has: ID, Name, Provider, Tier (uppercase from Go JSON)
        const allModels = [];
        for (const [tier, tierModels] of Object.entries(res)) {
          if (Array.isArray(tierModels)) {
            for (const m of tierModels) {
              allModels.push({
                id: m.ID || m.id || m.Name || m.name || '',
                name: m.Name || m.name || m.ID || m.id || '',
                tier: m.Tier || m.tier || tier.charAt(0).toUpperCase() + tier.slice(1),
                provider: m.Provider || m.provider || tier,
              });
            }
          }
        }
        models = allModels;
      } else {
        models = [];
      }
    } catch (err) {
      error = err.message || 'Failed to load models';
    } finally {
      loading = false;
    }
  }

  onMount(fetchModels);

  let filteredModels = $derived(
    models.filter(model => {
      const modelId = (model.id || model.name || '').toLowerCase();
      const matchesSearch = modelId.includes(searchQuery.toLowerCase());
      const matchesFilter = selectedFilter === 'All' || 
        (model.tier || '').toLowerCase() === selectedFilter.toLowerCase();
      return matchesSearch && matchesFilter;
    })
  );
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold mb-1">Models</h1>
      <p class="text-text-muted text-sm">Available AI models across all tiers.</p>
    </div>
    <div class="relative w-full sm:w-64">
      <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-text-muted">
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>
      </div>
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Search models..."
        class="w-full bg-bg-base border border-border rounded-lg pl-10 pr-4 py-2 text-sm placeholder:text-text-muted focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors"
      />
    </div>
  </div>

  <!-- Filter pills -->
  <div class="flex flex-wrap gap-2">
    {#each filters as filter}
      <button
        class="px-3 py-1.5 text-sm font-medium rounded-lg transition-colors {selectedFilter === filter ? 'bg-accent text-white' : 'bg-bg-sidebar border border-border text-text-muted hover:text-text-base hover:bg-sidebar-hover'}"
        onclick={() => selectedFilter = filter}
      >
        {filter}
      </button>
    {/each}
  </div>

  <!-- Table -->
  <div class="rounded-xl border border-border bg-bg-sidebar/50 overflow-hidden">
    {#if loading}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
        <p>Loading models...</p>
      </div>
    {:else if error}
      <div class="p-8 flex flex-col items-center justify-center text-red-400 space-y-4 min-h-[300px]">
        <p>{error}</p>
        <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-text-base hover:bg-sidebar-hover transition-colors" onclick={fetchModels}>
          Retry
        </button>
      </div>
    {:else if filteredModels.length === 0}
      <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
        <p>No models found.</p>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="border-b border-border text-xs uppercase text-text-muted">
            <tr>
              <th class="px-4 py-3 font-medium">MODEL</th>
              <th class="px-4 py-3 font-medium">TIER</th>
              <th class="px-4 py-3 font-medium">PROVIDER</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each filteredModels as model}
              <tr class="transition-colors hover:bg-accent/5">
                <td class="px-4 py-3">
                  <span class="font-mono text-sm bg-bg-base px-2 py-1 rounded border border-border">{model.id || model.name}</span>
                </td>
                <td class="px-4 py-3">
                  <Badge text={model.tier || 'Standard'} color={tierColors[model.tier] || 'gray'} />
                </td>
                <td class="px-4 py-3 text-text-muted">
                  {model.provider || '-'}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>
