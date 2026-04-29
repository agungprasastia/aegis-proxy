<script>
  import { fade, fly } from 'svelte/transition';

  let {
    isOpen = $bindable(false),
    title = '',
    onClose = () => {},
    children
  } = $props();

  function handleClose() {
    isOpen = false;
    onClose();
  }

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  }
</script>

{#if isOpen}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    onmousedown={handleBackdropClick}
    onkeydown={(e) => e.key === 'Escape' && handleClose()}
    role="dialog"
    aria-modal="true"
    tabindex="-1"
  >
    <!-- Backdrop -->
    <div 
      class="absolute inset-0 bg-black/60 backdrop-blur-sm"
      transition:fade={{ duration: 200 }}
    ></div>

    <!-- Modal -->
    <div 
      class="relative w-full max-w-lg rounded-xl border border-border bg-bg-sidebar shadow-2xl flex flex-col max-h-[90vh]"
      transition:fly={{ y: 20, duration: 200 }}
    >
      <!-- Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-border shrink-0">
        <h2 class="text-lg font-semibold">{title}</h2>
        <button
          class="p-1 text-text-muted hover:text-text-base rounded-md hover:bg-sidebar-hover transition-colors"
          onclick={handleClose}
          aria-label="Close modal"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M18 6 6 18"/>
            <path d="m6 6 12 12"/>
          </svg>
        </button>
      </div>

      <!-- Body -->
      <div class="overflow-y-auto min-h-0">
        {@render children?.()}
      </div>
    </div>
  </div>
{/if}
