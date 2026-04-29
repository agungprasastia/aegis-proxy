<script>
  import Modal from './components/Modal.svelte';
  import { api } from './api.js';

  let { provider = $bindable(''), show = $bindable(false), onclose = () => {}, onstart = () => {} } = $props();

  let accountsText = $state('');
  let concurrent = $state(1);
  let headless = $state(true);
  let priority = $state('standard');
  let loading = $state(false);
  let message = $state('');
  let messageType = $state('');

  let accountCount = $derived(
    accountsText.trim().split('\n').filter(l => l.trim() && (l.includes(':') || l.includes('|'))).length
  );

  async function handleStart() {
    const lines = accountsText.trim().split('\n').filter(l => l.trim());
    const accounts = [];

    for (const line of lines) {
      const parts = line.trim().split(/[:|]/);
      if (parts.length >= 2) {
        accounts.push({ email: parts[0].trim(), password: parts[1].trim() });
      }
    }

    if (accounts.length === 0) {
      message = 'Please enter at least one account (email:password)';
      messageType = 'error';
      return;
    }

    loading = true;
    message = '';

    try {
      // Save settings
      await api.put('/api/settings', {
        account_add_concurrent: concurrent,
        account_add_headless: headless,
        account_add_priority: priority
      });

      // Start batch login
      await api.post('/api/batch/start', { accounts });

      // Navigate to progress page
      show = false;
      accountsText = '';
      onstart();
    } catch (e) {
      message = e.message || 'Failed to start batch';
      messageType = 'error';
    } finally {
      loading = false;
    }
  }

  function handleClose() {
    show = false;
    accountsText = '';
    message = '';
    messageType = '';
    onclose();
  }
</script>

<Modal bind:isOpen={show} title={`Add ${provider.charAt(0).toUpperCase() + provider.slice(1)} Accounts`} onClose={handleClose}>
  <div class="px-6 py-4 space-y-4">
    <!-- Accounts textarea -->
    <div>
      <label for="accounts-input" class="block text-sm font-medium text-text-muted mb-1">
        Accounts <span class="text-text-muted/50">(email:password, one per line)</span>
      </label>
      <textarea
        id="accounts-input"
        bind:value={accountsText}
        placeholder={"email1@gmail.com:password123\nemail2@gmail.com:password456"}
        rows="6"
        disabled={loading}
        class="w-full rounded-lg border border-border bg-bg-base px-3 py-2 text-sm font-mono placeholder-text-muted/40 focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent resize-none disabled:opacity-50"
      ></textarea>
    </div>

    <!-- Options -->
    <div class="flex flex-wrap items-center gap-4">
      <div class="flex items-center gap-2">
        <label for="concurrent" class="text-sm text-text-muted">Concurrent</label>
        <select id="concurrent" bind:value={concurrent} disabled={loading} class="bg-bg-base border border-border rounded-lg px-2 py-1.5 text-sm focus:outline-none focus:border-accent">
          <option value={1}>1 at a time</option>
          <option value={2}>2 parallel</option>
          <option value={3}>3 parallel</option>
          <option value={4}>4 parallel</option>
        </select>
      </div>
      <label class="flex items-center gap-2 cursor-pointer text-sm text-text-muted">
        <input type="checkbox" bind:checked={headless} disabled={loading} class="rounded border-border" />
        Headless
      </label>
    </div>

    {#if message}
      <div class="text-sm p-3 rounded-lg {messageType === 'error' ? 'bg-red-400/10 text-red-400 border border-red-400/20' : 'bg-green-400/10 text-green-400 border border-green-400/20'}">
        {message}
      </div>
    {/if}

    <!-- Buttons -->
    <div class="flex items-center justify-end gap-3 pt-2 border-t border-border">
      <button class="px-4 py-2 text-sm text-text-muted hover:text-text-base transition-colors" onclick={handleClose} disabled={loading}>
        Cancel
      </button>
      <button
        class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 flex items-center gap-2 min-h-[40px]"
        onclick={handleStart}
        disabled={loading || accountCount === 0}
      >
        {#if loading}
          <div class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
          Starting...
        {:else}
          Start ({accountCount} accounts)
        {/if}
      </button>
    </div>
  </div>
</Modal>
