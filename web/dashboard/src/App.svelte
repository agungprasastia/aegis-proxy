<script>
  import DashboardPage from './lib/DashboardPage.svelte';
  import AccountsPage from './lib/AccountsPage.svelte';
  import BatchProgressPage from './lib/BatchProgressPage.svelte';
  import ModelsPage from './lib/ModelsPage.svelte';
  import ProxyPage from './lib/ProxyPage.svelte';
  import CombosPage from './lib/CombosPage.svelte';
  import LogsPage from './lib/LogsPage.svelte';
  import ApiKeyPage from './lib/ApiKeyPage.svelte';
  import ProvidersPage from './lib/ProvidersPage.svelte';
  import SettingsPage from './lib/SettingsPage.svelte';
  import FiltersPage from './lib/FiltersPage.svelte';
  import LoginPage from './lib/LoginPage.svelte';
  import Toast from './lib/Toast.svelte';
  import { auth } from './lib/stores/auth.js';
  import { api } from './lib/api.js';

  const routePaths = {
    Dashboard: '/dashboard',
    Accounts: '/dashboard/accounts',
    Models: '/dashboard/models',
    'API Key': '/dashboard/api-key',
    Providers: '/dashboard/providers',
    Proxy: '/dashboard/proxy',
    Filters: '/dashboard/filters',
    Combos: '/dashboard/combos',
    Logs: '/dashboard/logs',
    Settings: '/dashboard/settings',
  };

  const pathRoutes = Object.fromEntries(Object.entries(routePaths).map(([name, path]) => [path, name]));

  function routeFromPath() {
    if (typeof window === 'undefined') return 'Dashboard';
    return pathRoutes[window.location.pathname] || 'Dashboard';
  }

  let activeRoute = $state(routeFromPath());
  let isMobileMenuOpen = $state(false);
  let proxyStatus = $state('checking');
  let theme = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('aegis_theme') || 'dark') : 'dark');

  // Apply theme on init
  $effect(() => {
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute('data-theme', theme);
    }
  });

  // Collapsible sidebar sections
  let sectionState = $state(
    typeof localStorage !== 'undefined'
      ? JSON.parse(localStorage.getItem('aegis_sidebar_sections') || '{"accounts":true,"proxy":true,"logs":true}')
      : { accounts: true, proxy: true, logs: true }
  );

  function toggleSection(key) {
    sectionState[key] = !sectionState[key];
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('aegis_sidebar_sections', JSON.stringify(sectionState));
    }
  }

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark';
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute('data-theme', theme);
    }
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('aegis_theme', theme);
    }
  }

  // Poll health endpoint every 30s
  async function checkHealth() {
    try {
      const baseUrl = typeof window !== 'undefined' && window.location.port === '5173'
        ? 'http://localhost:3131'
        : '';
      const res = await fetch(`${baseUrl}/health`);
      if (res.ok) {
        proxyStatus = 'running';
      } else {
        proxyStatus = 'stopped';
      }
    } catch {
      proxyStatus = 'stopped';
    }
  }

  // Start health polling
  if (typeof window !== 'undefined') {
    checkHealth();
    setInterval(checkHealth, 30000);
  }

  const navSections = [
    {
      key: 'accounts',
      label: 'ACCOUNTS',
      items: [
        { name: 'Dashboard', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="9" x="3" y="3" rx="1"/><rect width="7" height="5" x="14" y="3" rx="1"/><rect width="7" height="9" x="14" y="12" rx="1"/><rect width="7" height="5" x="3" y="16" rx="1"/></svg>' },
        { name: 'Accounts', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>' },
        { name: 'Models', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>' },
      ]
    },
    {
      key: 'proxy',
      label: 'PROXY',
      items: [
        { name: 'API Key', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 18v3c0 .6.4 1 1 1h4v-3h3v-3h2l1.4-1.4a6.5 6.5 0 1 0-4-4Z"/><circle cx="16.5" cy="7.5" r=".5"/></svg>' },
        { name: 'Providers', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v18"/><rect width="18" height="14" x="3" y="5" rx="2"/><path d="M7 9h.01"/><path d="M7 13h.01"/><path d="M17 9h.01"/><path d="M17 13h.01"/></svg>' },
        { name: 'Proxy', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="16" height="16" x="4" y="4" rx="2"/><rect width="6" height="6" x="9" y="9" rx="1"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="M2 12h2"/><path d="M20 12h2"/></svg>' },
        { name: 'Filters', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>' },
        { name: 'Combos', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 17h6"/><path d="M14 17h6"/><path d="M10 17a2 2 0 1 0 4 0 2 2 0 0 0-4 0"/><path d="M4 7h10"/><path d="M18 7h2"/><path d="M14 7a2 2 0 1 0 4 0 2 2 0 0 0-4 0"/></svg>' },
      ]
    },
    {
      key: 'logs',
      label: 'LOGS',
      items: [
        { name: 'Logs', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>' },
        { name: 'Login', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><polyline points="10 17 15 12 10 7"/><line x1="15" y1="12" x2="3" y2="12"/></svg>' },
      ]
    }
  ];

  const flatItems = [
    { name: 'Settings', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>' },
  ];

  function navigate(name) {
    activeRoute = name;
    isMobileMenuOpen = false;
    if (typeof window !== 'undefined' && routePaths[name] && window.location.pathname !== routePaths[name]) {
      window.history.pushState({}, '', routePaths[name]);
    }
  }

  function handleLogout() {
    auth.logout();
  }

  if (typeof window !== 'undefined') {
    window.addEventListener('popstate', () => {
      activeRoute = routeFromPath();
    });
  }
</script>

{#if !$auth.isAuthenticated}
  <LoginPage />
{:else}
  <div class="flex h-screen bg-bg-base text-text-base overflow-hidden">
    
    <!-- Mobile Sidebar Overlay -->
    {#if isMobileMenuOpen}
      <div 
        class="fixed inset-0 bg-black/50 z-20 lg:hidden transition-opacity"
        onclick={() => isMobileMenuOpen = false}
        aria-hidden="true"
      ></div>
    {/if}

    <!-- Sidebar -->
    <aside 
      class={`fixed inset-y-0 left-0 z-30 w-64 bg-bg-sidebar border-r border-border transform transition-transform duration-300 ease-in-out lg:translate-x-0 lg:static lg:block flex flex-col ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full'}`}
    >
      <!-- Logo area -->
      <div class="flex items-center h-16 px-6 border-b border-border shrink-0">
        <div class="flex items-center gap-2 font-bold text-xl tracking-tight text-white">
          <div class="w-8 h-8 rounded-lg bg-accent flex items-center justify-center text-white shadow-[0_0_15px_rgba(102,126,234,0.5)]">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
          </div>
          <span>aegis</span>
        </div>
      </div>

      <!-- Navigation with collapsible sections -->
      <nav class="flex-1 overflow-y-auto py-3 px-3 space-y-1">
        {#each navSections as section}
          <!-- Section header -->
          <button
            class="w-full flex items-center justify-between px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-text-muted hover:text-text-base transition-colors"
            onclick={() => toggleSection(section.key)}
          >
            <span>{section.label}</span>
            <svg 
              xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
              class="transition-transform duration-200 {sectionState[section.key] ? 'rotate-0' : '-rotate-90'}"
            ><polyline points="6 9 12 15 18 9"></polyline></svg>
          </button>

          <!-- Section items -->
          {#if sectionState[section.key]}
            <div class="space-y-0.5 pb-2">
              {#each section.items as item}
                <button
                  class={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 min-h-[44px] ${
                    activeRoute === item.name 
                      ? 'bg-accent/15 text-accent shadow-[inset_3px_0_0_var(--color-accent)]' 
                      : 'text-text-muted hover:text-text-base hover:bg-sidebar-hover'
                  }`}
                  onclick={() => navigate(item.name)}
                >
                  <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                  {@html item.icon}
                  {item.name}
                </button>
              {/each}
            </div>
          {/if}
        {/each}

        <!-- Flat items (no section) -->
        <div class="pt-2 border-t border-border space-y-0.5">
          {#each flatItems as item}
            <button
              class={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 min-h-[44px] ${
                activeRoute === item.name 
                  ? 'bg-accent/15 text-accent shadow-[inset_3px_0_0_var(--color-accent)]' 
                  : 'text-text-muted hover:text-text-base hover:bg-sidebar-hover'
              }`}
              onclick={() => navigate(item.name)}
            >
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              {@html item.icon}
              {item.name}
            </button>
          {/each}
        </div>
      </nav>
      
      <!-- Sidebar Footer: Status + Version + Theme + Logout -->
      <div class="p-3 border-t border-border shrink-0 space-y-2">
        <!-- Proxy Status -->
        <div class="flex items-center justify-between px-3 py-1.5 text-xs text-text-muted">
          <div class="flex items-center gap-2">
            {#if proxyStatus === 'running'}
              <div class="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)]"></div>
              <span>Proxy running</span>
            {:else if proxyStatus === 'stopped'}
              <div class="w-2 h-2 rounded-full bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.6)]"></div>
              <span>Proxy stopped</span>
            {:else}
              <div class="w-2 h-2 rounded-full bg-yellow-500 animate-pulse"></div>
              <span>Checking...</span>
            {/if}
          </div>
          <span class="text-text-muted/60">v1.0.0</span>
        </div>

        <!-- Theme toggle + Logout -->
        <div class="flex items-center justify-between px-2">
          <!-- Theme toggle -->
          <button
            onclick={toggleTheme}
            class="flex items-center gap-2 px-2 py-1.5 text-xs text-text-muted hover:text-text-base rounded-md hover:bg-sidebar-hover transition-colors min-h-[36px]"
            title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            aria-label="Toggle theme"
          >
            {#if theme === 'dark'}
              <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
              <span>Light</span>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
              <span>Dark</span>
            {/if}
          </button>

          <!-- Logout -->
          <button 
            onclick={handleLogout}
            class="flex items-center gap-2 px-2 py-1.5 text-xs text-text-muted hover:text-red-400 rounded-md hover:bg-red-400/10 transition-colors min-h-[36px]"
            title="Log out"
            aria-label="Log out"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/></svg>
            <span>Logout</span>
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 flex flex-col min-w-0 overflow-hidden">
      <!-- Header -->
      <header class="h-14 flex items-center justify-between px-4 lg:px-8 border-b border-border bg-bg-base/80 backdrop-blur-sm z-10 sticky top-0 shrink-0">
        <div class="flex items-center gap-4">
          <!-- Hamburger Menu (mobile) -->
          <button 
            class="lg:hidden p-2 -ml-2 text-text-muted hover:text-text-base rounded-md hover:bg-sidebar-hover transition-colors min-h-[44px] min-w-[44px] flex items-center justify-center"
            onclick={() => isMobileMenuOpen = true}
            aria-label="Open sidebar"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/></svg>
          </button>
          <h1 class="text-lg font-semibold">{activeRoute}</h1>
        </div>
        
        <div class="flex items-center gap-3">
          <!-- Status Indicator (desktop) -->
          <div class="items-center gap-2 text-sm text-text-muted hidden sm:flex">
            {#if proxyStatus === 'running'}
              <div class="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.6)]"></div>
              <span>Active</span>
            {:else}
              <div class="w-2 h-2 rounded-full bg-red-500"></div>
              <span>Offline</span>
            {/if}
          </div>
        </div>
      </header>

      <!-- Page Content -->
      <div class="flex-1 overflow-auto p-4 lg:p-8">
        <div class="max-w-6xl mx-auto">
          {#if activeRoute === 'Dashboard'}
            <DashboardPage />
          {:else if activeRoute === 'Accounts'}
            <AccountsPage onBatchStart={() => navigate('BatchProgress')} />
          {:else if activeRoute === 'BatchProgress' || activeRoute === 'Login'}
            <BatchProgressPage onback={() => navigate('Accounts')} />
          {:else if activeRoute === 'Models'}
            <ModelsPage />
          {:else if activeRoute === 'Proxy'}
            <ProxyPage />
          {:else if activeRoute === 'Filters'}
            <FiltersPage />
          {:else if activeRoute === 'Combos'}
            <CombosPage />
          {:else if activeRoute === 'Logs'}
            <LogsPage />
          {:else if activeRoute === 'API Key'}
            <ApiKeyPage />
          {:else if activeRoute === 'Providers'}
            <ProvidersPage />
          {:else if activeRoute === 'Settings'}
            <SettingsPage />
          {:else}
            <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-8 min-h-[400px] flex flex-col items-center justify-center text-center">
              <h2 class="text-2xl font-semibold mb-2">{activeRoute}</h2>
              <p class="text-text-muted max-w-md">This page is coming soon.</p>
            </div>
          {/if}
        </div>
      </div>
    </main>
  </div>
{/if}

<Toast />
