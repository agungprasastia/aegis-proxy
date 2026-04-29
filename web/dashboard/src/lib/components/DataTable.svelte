<script>
  let {
    columns = [],
    data = [],
    loading = false,
    emptyMessage = "No data available"
  } = $props();

  const skeletonRows = Array.from({ length: 5 });
</script>

<div class="w-full overflow-x-auto">
  <table class="w-full text-left text-sm text-text-base">
    <thead class="border-b border-border text-xs uppercase text-text-muted">
      <tr>
        {#each columns as col}
          <th scope="col" class="px-4 py-3 font-medium" style={col.width ? `width: ${col.width}` : ''}>
            {col.label}
          </th>
        {/each}
      </tr>
    </thead>
    <tbody class="divide-y divide-border">
      {#if loading}
        {#each skeletonRows as _}
          <tr class="animate-pulse">
            {#each columns as col}
              <td class="px-4 py-3">
                <div class="h-4 w-3/4 rounded bg-border"></div>
              </td>
            {/each}
          </tr>
        {/each}
      {:else if data.length === 0}
        <tr>
          <td colspan={Math.max(1, columns.length)} class="px-4 py-8 text-center text-text-muted">
            {emptyMessage}
          </td>
        </tr>
      {:else}
        {#each data as row}
          <tr class="transition-colors hover:bg-accent/5">
            {#each columns as col}
              <td class="px-4 py-3">
                {#if row[col.key] !== undefined && row[col.key] !== null}
                  {row[col.key]}
                {:else}
                  <span class="text-text-muted">-</span>
                {/if}
              </td>
            {/each}
          </tr>
        {/each}
      {/if}
    </tbody>
  </table>
</div>
