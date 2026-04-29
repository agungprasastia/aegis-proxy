<script>
  import Modal from './Modal.svelte';

  let {
    isOpen = $bindable(false),
    title = 'Confirm',
    message = 'Are you sure?',
    confirmText = 'Confirm',
    confirmVariant = 'default',
    isDanger = false,
    loading = false,
    onConfirm = () => {},
    onCancel = () => {},
    onClose = () => {},
  } = $props();

  // isDanger or confirmVariant="danger" both work
  let isDestructive = $derived(isDanger || confirmVariant === 'danger');

  function handleCancel() {
    isOpen = false;
    onCancel();
    onClose();
  }

  function handleConfirm() {
    onConfirm();
    if (!loading) {
      isOpen = false;
    }
  }
</script>

<Modal bind:isOpen {title} onClose={handleCancel}>
  <div class="px-6 py-4">
    <p class="text-sm text-text-muted">{message}</p>
  </div>

  <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-border">
    <button
      class="px-4 py-2 text-sm font-medium text-text-muted hover:text-text-base rounded-lg hover:bg-sidebar-hover transition-colors"
      onclick={handleCancel}
      disabled={loading}
    >
      Cancel
    </button>
    <button
      class="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors disabled:opacity-50 {isDestructive ? 'bg-red-600 hover:bg-red-700' : 'bg-accent hover:bg-accent-hover'}"
      onclick={handleConfirm}
      disabled={loading}
    >
      {#if loading}
        <span class="flex items-center gap-2">
          <svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
          Processing...
        </span>
      {:else}
        {confirmText}
      {/if}
    </button>
  </div>
</Modal>
