<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import Badge from './components/Badge.svelte';
  import ConfirmModal from './components/ConfirmModal.svelte';
  import { toast } from './stores/toast.js';

  let combos = $state([]);
  let models = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let validationError = $state('');
  let saving = $state(false);
  let deleting = $state(false);
  let showDeleteConfirm = $state(false);
  let comboToDelete = $state(null);
  let form = $state({ id: null, name: '', targets: [] });
  let selectedTarget = $state('');

  const tierColors = {
    standard: 'green',
    max: 'purple',
    wavespeed: 'blue',
    codex: 'orange',
    combo: 'gray',
  };

  let targetOptions = $derived([
    ...models.map((model) => ({
      provider: model.provider,
      model: model.id || model.name,
      label: `${model.provider} / ${model.id || model.name}`,
    })),
    ...combos
      .filter((combo) => combo.name !== form.name)
      .map((combo) => ({ provider: 'combo', model: combo.name, label: `combo / ${combo.name}` })),
  ]);

  function normalizeModels(res) {
    if (Array.isArray(res)) return res;
    if (Array.isArray(res?.data)) return res.data;
    if (!res || typeof res !== 'object') return [];

    const allModels = [];
    for (const [tier, tierModels] of Object.entries(res)) {
      if (!Array.isArray(tierModels)) continue;
      for (const model of tierModels) {
        allModels.push({
          id: model.ID || model.id || model.Name || model.name || '',
          name: model.Name || model.name || model.ID || model.id || '',
          provider: model.Provider || model.provider || tier,
          tier: model.Tier || model.tier || tier,
        });
      }
    }
    return allModels;
  }

  async function fetchCombos(showLoading = true) {
    try {
      if (showLoading) loading = true;
      error = null;
      const [comboRes, modelRes] = await Promise.all([
        api.get('/api/combos'),
        api.get('/api/models'),
      ]);
      combos = Array.isArray(comboRes) ? comboRes : (Array.isArray(comboRes?.data) ? comboRes.data : []);
      models = normalizeModels(modelRes);
    } catch (err) {
      error = err.message || 'Failed to load combos';
    } finally {
      loading = false;
    }
  }

  function resetForm() {
    form = { id: null, name: '', targets: [] };
    selectedTarget = '';
    validationError = '';
  }

  function editCombo(combo) {
    form = {
      id: combo.id,
      name: combo.name || '',
      targets: (combo.targets || []).map((target, index) => ({
        provider: target.provider || '',
        model: target.model || '',
        priority: target.priority || index + 1,
      })).sort((a, b) => a.priority - b.priority),
    };
    selectedTarget = '';
    validationError = '';
  }

  function addTarget() {
    const option = targetOptions.find((target) => `${target.provider}:${target.model}` === selectedTarget);
    if (!option) return;
    if (form.targets.some((target) => target.provider === option.provider && target.model === option.model)) {
      validationError = `Duplicate combo target: ${option.provider}:${option.model}`;
      return;
    }
    form.targets = [...form.targets, { provider: option.provider, model: option.model, priority: form.targets.length + 1 }];
    selectedTarget = '';
    validationError = '';
  }

  function removeTarget(index) {
    form.targets = form.targets.filter((_, targetIndex) => targetIndex !== index).map((target, targetIndex) => ({ ...target, priority: targetIndex + 1 }));
  }

  function moveTarget(index, direction) {
    const nextIndex = index + direction;
    if (nextIndex < 0 || nextIndex >= form.targets.length) return;
    const targets = [...form.targets];
    [targets[index], targets[nextIndex]] = [targets[nextIndex], targets[index]];
    form.targets = targets.map((target, targetIndex) => ({ ...target, priority: targetIndex + 1 }));
  }

  function validateForm() {
    if (!form.name.trim()) return 'Combo name is required';
    if (form.targets.length === 0) return 'Combo target chain is empty';
    if (form.targets.some((target) => target.provider === 'combo' && target.model === form.name.trim())) {
      return `Recursive combo target: ${form.name.trim()}`;
    }
    return '';
  }

  async function saveCombo() {
    validationError = validateForm();
    if (validationError) return;

    const payload = {
      name: form.name.trim(),
      targets: form.targets.map((target, index) => ({ ...target, priority: index + 1 })),
    };

    try {
      saving = true;
      if (form.id) {
        await api.put(`/api/combos/${form.id}`, { ...payload, id: form.id });
        toast.success('Combo updated');
      } else {
        await api.post('/api/combos', payload);
        toast.success('Combo created');
      }
      resetForm();
      await fetchCombos(false);
    } catch (err) {
      validationError = err.message || 'Failed to save combo';
      toast.error(validationError);
    } finally {
      saving = false;
    }
  }

  async function deleteCombo() {
    if (!comboToDelete) return;
    try {
      deleting = true;
      await api.delete(`/api/combos/${comboToDelete.id}`);
      toast.success('Combo deleted');
      if (form.id === comboToDelete.id) resetForm();
      comboToDelete = null;
      showDeleteConfirm = false;
      await fetchCombos(false);
    } catch (err) {
      toast.error(err.message || 'Failed to delete combo');
    } finally {
      deleting = false;
    }
  }

  onMount(fetchCombos);
</script>

<div class="space-y-6">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold mb-1">Combos</h1>
      <p class="text-text-muted text-sm">Ordered fallback chains across providers, models, and other combos.</p>
    </div>
    <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-sm text-text-base hover:bg-sidebar-hover transition-colors" onclick={resetForm}>New Combo</button>
  </div>

  <div class="grid gap-6 xl:grid-cols-[1fr_420px]">
    <div class="rounded-xl border border-border bg-bg-sidebar/50 overflow-hidden">
      {#if loading}
        <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
          <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
          <p>Loading combos...</p>
        </div>
      {:else if error}
        <div class="p-8 flex flex-col items-center justify-center text-red-400 space-y-4 min-h-[300px]">
          <p>{error}</p>
          <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-text-base hover:bg-sidebar-hover transition-colors" onclick={() => fetchCombos()}>Retry</button>
        </div>
      {:else if combos.length === 0}
        <div class="p-8 flex flex-col items-center justify-center text-text-muted space-y-4 min-h-[300px]">
          <p>No combos created yet.</p>
        </div>
      {:else}
        <div class="divide-y divide-border">
          {#each combos as combo}
            <div class="p-4 hover:bg-accent/5 transition-colors">
              <div class="flex items-start justify-between gap-4 mb-3">
                <div>
                  <h2 class="font-semibold">{combo.name}</h2>
                  <p class="text-xs text-text-muted">{(combo.targets || []).length} ordered targets</p>
                </div>
                <div class="flex items-center gap-2">
                  <button class="px-3 py-1.5 text-xs bg-bg-base border border-border rounded-lg hover:bg-sidebar-hover transition-colors" onclick={() => editCombo(combo)}>Edit</button>
                  <button class="px-3 py-1.5 text-xs text-red-400 bg-red-400/10 rounded-lg hover:bg-red-400/20 transition-colors" onclick={() => { comboToDelete = combo; showDeleteConfirm = true; }}>Delete</button>
                </div>
              </div>
              <div class="space-y-2">
                {#each (combo.targets || []).slice().sort((a, b) => a.priority - b.priority) as target}
                  <div class="flex items-center gap-2 rounded-lg bg-bg-base border border-border px-3 py-2 text-sm">
                    <span class="w-6 h-6 rounded-full bg-accent/15 text-accent flex items-center justify-center text-xs font-semibold">{target.priority}</span>
                    <Badge text={target.provider} color={tierColors[target.provider] || 'gray'} />
                    <span class="font-mono text-text-muted truncate">{target.model}</span>
                  </div>
                {/each}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="rounded-xl border border-border bg-bg-sidebar/50 p-5 h-fit space-y-4">
      <div>
        <h2 class="text-lg font-semibold">{form.id ? 'Edit Combo' : 'Create Combo'}</h2>
        <p class="text-sm text-text-muted">Targets run in priority order.</p>
      </div>

      {#if validationError}
        <div class="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">{validationError}</div>
      {/if}

      <div>
        <label for="comboName" class="block text-sm font-medium text-text-muted mb-1">Name</label>
        <input id="comboName" type="text" bind:value={form.name} placeholder="standard-fallback" class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 font-mono text-sm placeholder:text-text-muted/50 focus:outline-none focus:border-accent transition-colors" />
      </div>

      <div class="space-y-2">
        <label for="comboTarget" class="block text-sm font-medium text-text-muted">Add Target</label>
        <div class="flex gap-2">
          <select id="comboTarget" bind:value={selectedTarget} class="min-w-0 flex-1 bg-bg-base border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-accent transition-colors">
            <option value="">Select provider/model...</option>
            {#each targetOptions as option}
              <option value={`${option.provider}:${option.model}`}>{option.label}</option>
            {/each}
          </select>
          <button class="px-3 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50" onclick={addTarget} disabled={!selectedTarget}>Add</button>
        </div>
      </div>

      <div class="space-y-2">
        <h3 class="text-sm font-medium text-text-muted">Ordered Targets</h3>
        {#if form.targets.length === 0}
          <div class="rounded-lg border border-dashed border-border p-4 text-sm text-text-muted text-center">No targets added.</div>
        {:else}
          {#each form.targets as target, index}
            <div class="flex items-center gap-2 rounded-lg bg-bg-base border border-border p-2">
              <span class="w-7 h-7 rounded-full bg-accent/15 text-accent flex items-center justify-center text-xs font-semibold">{index + 1}</span>
              <Badge text={target.provider} color={tierColors[target.provider] || 'gray'} />
              <span class="min-w-0 flex-1 font-mono text-sm truncate">{target.model}</span>
              <button class="p-1.5 text-text-muted hover:text-text-base rounded hover:bg-sidebar-hover disabled:opacity-30" onclick={() => moveTarget(index, -1)} disabled={index === 0} title="Move up">↑</button>
              <button class="p-1.5 text-text-muted hover:text-text-base rounded hover:bg-sidebar-hover disabled:opacity-30" onclick={() => moveTarget(index, 1)} disabled={index === form.targets.length - 1} title="Move down">↓</button>
              <button class="p-1.5 text-text-muted hover:text-red-400 rounded hover:bg-red-400/10" onclick={() => removeTarget(index)} title="Remove target">Remove</button>
            </div>
          {/each}
        {/if}
      </div>

      <div class="flex justify-end gap-3 pt-2">
        <button class="px-4 py-2 text-sm text-text-muted hover:text-text-base transition-colors" onclick={resetForm}>Cancel</button>
        <button class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50" onclick={saveCombo} disabled={saving}>
          {saving ? 'Saving...' : (form.id ? 'Save Changes' : 'Create Combo')}
        </button>
      </div>
    </div>
  </div>

  <ConfirmModal
    bind:isOpen={showDeleteConfirm}
    title="Delete Combo"
    message={`Delete combo "${comboToDelete?.name || ''}"? This cannot be undone.`}
    confirmText="Delete"
    isDanger={true}
    loading={deleting}
    onConfirm={deleteCombo}
    onClose={() => comboToDelete = null}
  />
</div>
