<script>
  let {
    icon,
    label = '',
    value = '',
    subtitle = ''
  } = $props();
</script>

<div class="relative overflow-hidden rounded-xl border border-border bg-bg-sidebar p-6 transition-all duration-300 hover:-translate-y-1 hover:shadow-lg">
  <div class="flex items-center gap-5">
    {#if icon}
      <div class="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-bg-base text-accent shadow-inner ring-1 ring-white/5">
        <!-- eslint-disable-next-line svelte/no-at-html-tags -->
        {@html icon}
      </div>
    {/if}
    
    <div class="flex flex-col">
      <div class="text-3xl font-bold tracking-tight">{value}</div>
      <div class="mt-1 text-sm font-medium text-text-muted">{label}</div>
      {#if subtitle}
        <div class="mt-1.5 text-xs text-text-muted/60">{subtitle}</div>
      {/if}
    </div>
  </div>
</div>
