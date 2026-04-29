<script>
  let {
    tabs = [],
    activeTab = $bindable('')
  } = $props();
</script>

<div class="border-b border-border">
  <nav class="-mb-px flex space-x-6" aria-label="Tabs">
    {#each tabs as tab (tab.id)}
      <button
        onclick={() => { activeTab = tab.id; }}
        class="group flex items-center gap-2 border-b-2 px-1 py-3 text-sm font-medium transition-all duration-200 {activeTab === tab.id
          ? 'border-accent text-accent'
          : 'border-transparent text-text-muted hover:border-border hover:text-text-base'}"
        aria-current={activeTab === tab.id ? 'page' : undefined}
      >
        {tab.label}
        {#if tab.count !== undefined}
          <span
            class="inline-flex items-center justify-center rounded-full px-2 py-0.5 text-xs font-medium transition-colors {activeTab === tab.id
              ? 'bg-accent/20 text-accent'
              : 'bg-bg-sidebar border border-border text-text-muted'}"
          >
            {tab.count}
          </span>
        {/if}
      </button>
    {/each}
  </nav>
</div>
