<script>
  let {
    text = '',
    color = 'gray'
  } = $props();

  const colorVariants = {
    green: 'bg-green-500/20 text-green-400 border-green-500/30',
    orange: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
    red: 'bg-red-500/20 text-red-400 border-red-500/30',
    blue: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
    purple: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
    gray: 'bg-gray-500/20 text-gray-400 border-gray-500/30'
  };

  let colorClasses = $derived(colorVariants[color] || colorVariants.gray);
</script>

<span class="inline-flex items-center justify-center px-2.5 py-0.5 text-xs font-medium rounded-full border {colorClasses}">
  {text}
</span>
