<script>
  let { provider = $bindable(''), show = $bindable(false), onclose = () => {} } = $props();

  let accountsText = $state('');
  let loading = $state(false);
  let message = $state('');
  let messageType = $state('');

  async function handleSubmit() {
    if (!accountsText.trim()) {
      message = 'Please enter at least one account';
      messageType = 'error';
      return;
    }

    loading = true;
    message = '';
    messageType = '';

    try {
      const res = await fetch('/api/accounts', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('session_token') || ''}`
        },
        body: JSON.stringify({
          provider: provider,
          accounts: accountsText.trim()
        })
      });

      if (res.ok) {
        const data = await res.json();
        message = data.message || `Accounts added successfully`;
        messageType = 'success';
        accountsText = '';
        setTimeout(() => {
          handleClose();
        }, 1500);
      } else {
        const err = await res.json().catch(() => ({}));
        message = err.error || `Failed to add accounts (${res.status})`;
        messageType = 'error';
      }
    } catch (e) {
      message = 'Network error. Is the server running?';
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

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  }
</script>

{#if show}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    onclick={handleBackdropClick}
    onkeydown={(e) => e.key === 'Escape' && handleClose()}
  >
    <!-- Backdrop -->
    <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>

    <!-- Modal -->
    <div class="relative w-full max-w-lg rounded-xl border border-border bg-bg-sidebar shadow-2xl">
      <!-- Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-border">
        <h2 class="text-lg font-semibold text-white">Add Accounts to {provider}</h2>
        <button
          class="p-1 text-text-muted hover:text-white rounded-md hover:bg-sidebar-hover transition-colors"
          onclick={handleClose}
          aria-label="Close modal"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
        </button>
      </div>

      <!-- Body -->
      <div class="px-6 py-4 space-y-4">
        <div>
          <label for="accounts-input" class="block text-sm font-medium text-text-muted mb-2">
            Enter accounts (one per line, email:password format)
          </label>
          <textarea
            id="accounts-input"
            bind:value={accountsText}
            placeholder={"user1@gmail.com:password123\nuser2@gmail.com:mypassword456"}
            rows="8"
            disabled={loading}
            class="w-full rounded-lg border border-border bg-bg-base px-4 py-3 text-sm text-text-base placeholder-text-muted/50 focus:outline-none focus:ring-2 focus:ring-accent/50 focus:border-accent/50 resize-none font-mono disabled:opacity-50"
          ></textarea>
        </div>

        {#if message}
          <div class={`rounded-lg px-4 py-3 text-sm ${messageType === 'success' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-red-500/10 text-red-400 border border-red-500/20'}`}>
            {message}
          </div>
        {/if}
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-border">
        <button
          class="px-4 py-2 text-sm font-medium text-text-muted hover:text-white rounded-lg hover:bg-sidebar-hover transition-colors"
          onclick={handleClose}
          disabled={loading}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm font-medium text-white bg-accent hover:bg-accent-hover rounded-lg transition-colors flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          onclick={handleSubmit}
          disabled={loading}
        >
          {#if loading}
            <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Adding...
          {:else}
            Add Accounts
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
