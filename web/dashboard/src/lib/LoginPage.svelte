<script>
  import { auth } from './stores/auth.js';

  let password = $state('');
  let loading = $state(false);
  let error = $state(null);

  async function handleLogin(e) {
    e.preventDefault();
    if (!password) {
      error = 'Please enter a password';
      return;
    }

    loading = true;
    error = null;

    try {
      await auth.login(password);
      // App.svelte handles the redirect implicitly through $auth.isAuthenticated
    } catch (err) {
      error = err.message || 'Login failed. Please check your password.';
    } finally {
      loading = false;
    }
  }
</script>

<div class="min-h-screen bg-bg-base text-text-base flex items-center justify-center p-4">
  <div class="w-full max-w-md bg-bg-sidebar border border-border rounded-xl shadow-2xl p-8 relative overflow-hidden">
    <!-- Subtle decorative glow -->
    <div class="absolute -top-24 -right-24 w-48 h-48 bg-accent/10 rounded-full blur-3xl pointer-events-none"></div>
    <div class="absolute -bottom-24 -left-24 w-48 h-48 bg-accent/5 rounded-full blur-3xl pointer-events-none"></div>

    <div class="relative z-10">
      <div class="flex flex-col items-center mb-8">
        <div class="w-12 h-12 rounded-xl bg-accent flex items-center justify-center text-white shadow-[0_0_20px_rgba(102,126,234,0.5)] mb-4">
          <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
        </div>
        <h1 class="text-2xl font-bold text-white tracking-tight">Aegis Dashboard</h1>
        <p class="text-text-muted text-sm mt-2">Enter your password to unlock</p>
      </div>

      <form onsubmit={handleLogin} class="space-y-6">
        <div class="space-y-2">
          <label for="password" class="block text-sm font-medium text-text-base">
            Dashboard Password
          </label>
          <div class="relative">
            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-text-muted">
              <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
            </div>
            <input
              id="password"
              type="password"
              bind:value={password}
              class="w-full bg-bg-base border border-border text-white text-sm rounded-lg focus:ring-1 focus:ring-accent focus:border-accent block pl-10 p-3 transition-colors outline-none placeholder-text-muted/50"
              placeholder="••••••••••••"
              disabled={loading}
              autocomplete="current-password"
            />
          </div>
        </div>

        {#if error}
          <div class="text-red-400 text-sm p-3 bg-red-400/10 border border-red-400/20 rounded-lg flex items-start gap-2">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="shrink-0 mt-0.5"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>
            <span>{error}</span>
          </div>
        {/if}

        <button
          type="submit"
          disabled={loading || !password}
          class="w-full text-white bg-accent hover:bg-accent-hover focus:ring-2 focus:ring-accent/50 font-medium rounded-lg text-sm px-5 py-3 text-center transition-all duration-200 shadow-lg shadow-accent/20 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center"
        >
          {#if loading}
            <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
            Unlocking...
          {:else}
            Unlock Dashboard
          {/if}
        </button>
      </form>
    </div>
  </div>
</div>