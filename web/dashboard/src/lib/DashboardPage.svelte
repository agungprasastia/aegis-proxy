<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import StatCard from './components/StatCard.svelte';
  import Badge from './components/Badge.svelte';

  let loading = $state(true);
  let error = $state(null);
  let stats = $state(null);
  let accounts = $state([]);

  async function loadData() {
    loading = true;
    error = null;
    try {
      const [statsRes, accountsRes] = await Promise.all([
        api.get('/api/dashboard/stats'),
        api.get('/api/accounts')
      ]);
      stats = statsRes || {};
      accounts = Array.isArray(accountsRes) ? accountsRes : [];
    } catch (e) {
      error = e.message || 'Failed to load dashboard data';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  const numberFormat = new Intl.NumberFormat('en-US');
  const percentFormat = new Intl.NumberFormat('en-US', { style: 'percent', maximumFractionDigits: 1 });

  function formatNumber(num) {
    return numberFormat.format(num || 0);
  }

  function formatPercent(num) {
    if (num > 1) num = num / 100; // Handle if API returns 99.9 instead of 0.999
    return percentFormat.format(num || 0);
  }

  // Derived values for StatCards
  let accountsValue = $derived(
    stats?.accounts 
      ? `${formatNumber(stats.accounts.active)} / ${formatNumber(stats.accounts.total)}`
      : '0 / 0'
  );
  
  let requestsValue = $derived(
    stats?.requests?.total !== undefined
      ? formatNumber(stats.requests.total)
      : '0'
  );

  let successRateValue = $derived(
    stats?.requests?.success_rate !== undefined
      ? formatPercent(stats.requests.success_rate)
      : '0%'
  );

  let uptimeValue = $derived(
    stats?.uptime || '0s'
  );

  // Derived values for Tiers
  // Tier mappings
  const tiers = [
    { id: 'standard', name: 'Standard', providers: ['kiro'] },
    { id: 'max', name: 'MAX', providers: ['codebuddy'] },
    { id: 'wavespeed', name: 'Wavespeed', providers: ['wavespeed'] }
  ];

  let tierData = $derived(
    tiers.map(tier => {
      const tierAccounts = accounts.filter(a => 
        a.provider && tier.providers.includes(a.provider.toLowerCase())
      );
      
      const totalAccounts = tierAccounts.length;
      const activeAccounts = tierAccounts.filter(a => a.status?.toLowerCase() === 'active').length;
      
      const creditsUsed = tierAccounts.reduce((sum, a) => sum + (Number(a.credits_used) || 0), 0);
      const creditsTotal = tierAccounts.reduce((sum, a) => sum + (Number(a.credits_total) || 0), 0);
      
      const usagePercent = creditsTotal > 0 ? (creditsUsed / creditsTotal) * 100 : 0;
      
      return {
        ...tier,
        totalAccounts,
        activeAccounts,
        creditsUsed,
        creditsTotal,
        usagePercent: Math.min(100, Math.max(0, usagePercent))
      };
    })
  );

  // SVG Icons
  const usersIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>`;
  const activityIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>`;
  const checkCircleIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>`;
  const clockIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>`;

</script>

<div class="space-y-8">
  <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
    <div>
      <h2 class="text-2xl font-bold text-white">Dashboard</h2>
      <p class="text-text-muted text-sm mt-1">Overview of proxy usage and account health</p>
    </div>
    {#if !loading && !error}
      <button
        onclick={loadData}
        class="px-3 py-2 text-sm font-medium rounded-lg border border-border text-text-muted hover:text-white hover:bg-sidebar-hover transition-colors flex items-center gap-2"
      >
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.2"/></svg>
        Refresh
      </button>
    {/if}
  </div>

  {#if loading}
    <!-- Loading Skeleton -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      {#each Array(4) as _}
        <div class="h-[120px] rounded-xl border border-border bg-bg-sidebar/50 animate-pulse"></div>
      {/each}
    </div>
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mt-8">
      {#each Array(3) as _}
        <div class="h-[200px] rounded-xl border border-border bg-bg-sidebar/50 animate-pulse"></div>
      {/each}
    </div>
  {:else if error}
    <!-- Error State -->
    <div class="rounded-xl border border-red-500/30 bg-red-500/10 p-8 text-center flex flex-col items-center justify-center min-h-[300px]">
      <svg class="h-12 w-12 text-red-400 mb-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
      </svg>
      <h3 class="text-xl font-semibold text-red-400 mb-2">Error Loading Dashboard</h3>
      <p class="text-text-muted mb-6">{error}</p>
      <button
        onclick={loadData}
        class="px-6 py-2.5 text-sm font-medium rounded-lg bg-accent text-white hover:bg-accent/90 transition-colors shadow-lg"
      >
        Try Again
      </button>
    </div>
  {:else}
    <!-- Content -->
    <!-- Stats Row -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <StatCard 
        icon={usersIcon}
        label="Accounts (Active / Total)"
        value={accountsValue}
      />
      <StatCard 
        icon={activityIcon}
        label="Total Requests"
        value={requestsValue}
      />
      <StatCard 
        icon={checkCircleIcon}
        label="Success Rate"
        value={successRateValue}
      />
      <StatCard 
        icon={clockIcon}
        label="Uptime"
        value={uptimeValue}
      />
    </div>

    <div class="mt-10">
      <h3 class="text-lg font-semibold text-white mb-4">Tier Overview</h3>
      
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        {#each tierData as tier}
          <div class="rounded-xl border border-border bg-[#161b22] p-6 flex flex-col gap-5 hover:border-[#30363d] transition-colors relative overflow-hidden group">
            <!-- Header -->
            <div class="flex items-center justify-between">
              <h4 class="text-lg font-semibold text-white">{tier.name}</h4>
              <Badge 
                text="{tier.activeAccounts} / {tier.totalAccounts} Active" 
                color={tier.activeAccounts > 0 ? "green" : "gray"}
              />
            </div>
            
            <!-- Credit Usage Numbers -->
            <div class="mt-2">
              <div class="flex justify-between text-sm mb-2">
                <span class="text-text-muted">Credits Used</span>
                <span class="font-medium text-white">
                  {formatNumber(tier.creditsUsed)} / {formatNumber(tier.creditsTotal)}
                </span>
              </div>
              
              <!-- Progress Bar -->
              <div class="h-2 w-full rounded-full bg-[#0d1117] overflow-hidden">
                <div 
                  class="h-full rounded-full transition-all duration-500 ease-out {tier.usagePercent > 90 ? 'bg-red-500' : tier.usagePercent > 75 ? 'bg-orange-500' : 'bg-accent'}"
                  style="width: {tier.usagePercent}%"
                ></div>
              </div>
              <div class="flex justify-between text-xs mt-1.5 text-text-muted">
                <span>{tier.usagePercent.toFixed(1)}%</span>
                {#if tier.creditsTotal > 0}
                  <span>{formatNumber(tier.creditsTotal - tier.creditsUsed)} remaining</span>
                {/if}
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>
