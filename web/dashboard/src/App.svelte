<script>
  let activeRoute = $state('Dashboard');
  let isMobileMenuOpen = $state(false);

  const navItems = [
    { name: 'Dashboard', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="9" x="3" y="3" rx="1"/><rect width="7" height="5" x="14" y="3" rx="1"/><rect width="7" height="9" x="14" y="12" rx="1"/><rect width="7" height="5" x="3" y="16" rx="1"/></svg>' },
    { name: 'Accounts', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>' },
    { name: 'Models', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>' },
    { name: 'Proxy', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="16" height="16" x="4" y="4" rx="2"/><rect width="6" height="6" x="9" y="9" rx="1"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="M2 12h2"/><path d="M20 12h2"/></svg>' },
    { name: 'Logs', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>' },
    { name: 'Tools', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>' },
    { name: 'Docs', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/></svg>' },
    { name: 'Settings', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>' },
    { name: 'Chat UI', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>' },
    { name: 'Donate', icon: '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/></svg>' }
  ];

  function navigate(name) {
    activeRoute = name;
    isMobileMenuOpen = false;
  }
</script>

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
    <div class="flex items-center h-16 px-6 border-b border-border">
      <div class="flex items-center gap-2 font-bold text-xl tracking-tight text-white">
        <div class="w-8 h-8 rounded-lg bg-accent flex items-center justify-center text-white shadow-[0_0_15px_rgba(102,126,234,0.5)]">
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
        </div>
        <span>aegis</span>
      </div>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 overflow-y-auto py-4 px-3 space-y-1">
      {#each navItems as item}
        <button
          class={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 ${
            activeRoute === item.name 
              ? 'bg-accent/15 text-accent shadow-[inset_2px_0_0_var(--color-accent)]' 
              : 'text-text-muted hover:text-text-base hover:bg-sidebar-hover'
          }`}
          onclick={() => navigate(item.name)}
        >
          <!-- eslint-disable-next-line svelte/no-at-html-tags -->
          {@html item.icon}
          {item.name}
        </button>
      {/each}
    </nav>
    
    <!-- User / Profile Footer Area (Optional) -->
    <div class="p-4 border-t border-border">
      <div class="flex items-center gap-3 px-3 py-2 text-sm text-text-muted rounded-lg hover:bg-sidebar-hover transition-colors cursor-pointer">
        <div class="w-8 h-8 rounded-full bg-border flex items-center justify-center">
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
        </div>
        <span>Admin User</span>
      </div>
    </div>
  </aside>

  <!-- Main Content -->
  <main class="flex-1 flex flex-col min-w-0 overflow-hidden">
    <!-- Header (Mobile Hamburger & Top Bar) -->
    <header class="h-16 flex items-center justify-between px-4 lg:px-8 border-b border-border bg-bg-base/80 backdrop-blur-sm z-10 sticky top-0">
      <div class="flex items-center gap-4">
        <!-- Hamburger Menu -->
        <button 
          class="lg:hidden p-2 -ml-2 text-text-muted hover:text-text-base rounded-md hover:bg-sidebar-hover transition-colors"
          onclick={() => isMobileMenuOpen = true}
          aria-label="Open sidebar"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="18" y2="18"/></svg>
        </button>
        <h1 class="text-xl font-semibold text-white">{activeRoute}</h1>
      </div>
      
      <div class="flex items-center gap-4">
        <!-- Status Indicator -->
        <div class="flex items-center gap-2 text-sm text-text-muted hidden sm:flex">
          <div class="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.6)]"></div>
          <span>Proxy Active</span>
        </div>
      </div>
    </header>

    <!-- Page Content -->
    <div class="flex-1 overflow-auto p-4 lg:p-8">
      <div class="max-w-6xl mx-auto">
        <!-- Placeholder content based on route -->
        <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-8 min-h-[400px] flex flex-col items-center justify-center text-center">
          <div class="w-16 h-16 rounded-2xl bg-accent/20 text-accent flex items-center justify-center mb-6">
            <!-- eslint-disable-next-line svelte/no-at-html-tags -->
            {@html navItems.find(i => i.name === activeRoute)?.icon}
          </div>
          <h2 class="text-2xl font-semibold text-white mb-2">{activeRoute} Overview</h2>
          <p class="text-text-muted max-w-md">
            This is the placeholder content for the {activeRoute} page. The actual implementation will be added in upcoming tasks.
          </p>
        </div>
      </div>
    </div>
  </main>
</div>
