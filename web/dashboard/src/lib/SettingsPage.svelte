<script>
  import { onMount } from 'svelte';
  import { api } from './api.js';
  import Tabs from './components/Tabs.svelte';
  import { toast } from './stores/toast.js';

  let loading = $state(true);
  let saving = $state(false);
  let error = $state(null);
  
  // Settings forms
  let currentPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let upstreamProxy = $state('');
  let rtkEnabled = $state(true);
  let syncEndpoint = $state('');
  
  let networkSettings = $state({
    proxy_host: '127.0.0.1',
    proxy_port: 3130,
    dashboard_host: '127.0.0.1',
    dashboard_port: 3131,
    expose_to_network: false,
    whitelisted_ips: []
  });

  const tabs = [
    { id: 'general', label: 'General' },
    { id: 'network', label: 'Network' },
    { id: 'rtk', label: 'RTK' },
    { id: 'sync', label: 'Sync' }
  ];
  let activeTab = $state('general');
  let ipWhitelistText = $state('');

  async function fetchSettings() {
    try {
      loading = true;
      error = null;
      const res = await api.get('/api/settings');
      if (res) {
        upstreamProxy = res.upstream_proxy || '';
        networkSettings = {
          proxy_host: res.proxy_host || '127.0.0.1',
          proxy_port: res.proxy_port || 3130,
          dashboard_host: res.dashboard_host || '127.0.0.1',
          dashboard_port: res.dashboard_port || 3131,
          expose_to_network: res.expose_to_network || false,
          whitelisted_ips: res.whitelisted_ips || []
        };
        rtkEnabled = res.rtk_enabled ?? true;
        syncEndpoint = res.sync_endpoint || '';
        ipWhitelistText = (res.whitelisted_ips || []).join(', ');
      }
    } catch (err) {
      error = err.message || 'Failed to load settings';
    } finally {
      loading = false;
    }
  }

  async function savePassword() {
    if (!currentPassword || !newPassword || !confirmPassword) {
      toast.error('All password fields are required');
      return;
    }
    
    if (newPassword !== confirmPassword) {
      toast.error('New passwords do not match');
      return;
    }

    try {
      saving = true;
      await api.put('/api/settings', { 
        dashboard_password: newPassword
      });
      toast.success('Password updated successfully');
      currentPassword = '';
      newPassword = '';
      confirmPassword = '';
    } catch (err) {
      toast.error(err.message || 'Failed to update password');
    } finally {
      saving = false;
    }
  }

  async function saveNetworkSettings() {
    try {
      saving = true;
      const ips = ipWhitelistText.split(',').map(s => s.trim()).filter(Boolean);
      await api.put('/api/settings', {
        proxy_host: networkSettings.proxy_host,
        proxy_port: networkSettings.proxy_port,
        dashboard_host: networkSettings.dashboard_host,
        dashboard_port: networkSettings.dashboard_port,
        expose_to_network: networkSettings.expose_to_network,
        whitelisted_ips: ips
      });
      toast.success('Network settings saved successfully');
    } catch (err) {
      toast.error(err.message || 'Failed to save network settings');
    } finally {
      saving = false;
    }
  }

  async function saveUpstreamProxy() {
    try {
      saving = true;
      await api.put('/api/settings', {
        upstream_proxy: upstreamProxy
      });
      toast.success('Upstream proxy saved successfully');
    } catch (err) {
      toast.error(err.message || 'Failed to save upstream proxy');
    } finally {
      saving = false;
    }
  }

  async function saveRTKSettings() {
    try {
      saving = true;
      await api.put('/api/settings', { rtk_enabled: rtkEnabled });
      toast.success('RTK settings saved');
    } catch (err) {
      toast.error(err.message || 'Failed to save RTK settings');
    } finally {
      saving = false;
    }
  }

  async function saveSyncSettings() {
    try {
      saving = true;
      await api.put('/api/settings', { sync_endpoint: syncEndpoint });
      toast.success('Sync settings saved');
    } catch (err) {
      toast.error(err.message || 'Failed to save sync settings');
    } finally {
      saving = false;
    }
  }

  onMount(fetchSettings);
</script>

<div class="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500 max-w-4xl mx-auto">
  <div>
    <h1 class="text-2xl font-bold text-white mb-1">Settings</h1>
    <p class="text-text-muted text-sm">Configure your proxy and dashboard preferences.</p>
  </div>

  {#if loading}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-8 flex flex-col items-center justify-center text-text-muted space-y-4">
      <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
      <p>Loading settings...</p>
    </div>
  {:else if error}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm p-8 flex flex-col items-center justify-center text-red-400 space-y-4">
      <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="opacity-50"><circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/></svg>
      <p>{error}</p>
      <button class="px-4 py-2 bg-bg-base border border-border rounded-lg text-text-base hover:bg-border transition-colors" onclick={fetchSettings}>
        Retry
      </button>
    </div>
  {:else}
    <div class="rounded-xl border border-border bg-bg-sidebar/50 backdrop-blur-sm overflow-hidden">
      <Tabs {tabs} bind:activeTab />
      
      <div class="p-6">
        {#if activeTab === 'general'}
          <div class="space-y-6 animate-in fade-in duration-300">
            <div>
              <h2 class="text-lg font-medium text-white mb-1">Dashboard Password</h2>
              <p class="text-sm text-text-muted mb-4">Change the password used to access this dashboard.</p>
              
              <div class="space-y-4 max-w-md">
                <div>
                  <label for="currentPassword" class="block text-sm font-medium text-text-muted mb-1">Current Password</label>
                  <input 
                    id="currentPassword"
                    type="password" 
                    bind:value={currentPassword}
                    class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors"
                  />
                </div>
                
                <div>
                  <label for="newPassword" class="block text-sm font-medium text-text-muted mb-1">New Password</label>
                  <input 
                    id="newPassword"
                    type="password" 
                    bind:value={newPassword}
                    class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors"
                  />
                </div>
                
                <div>
                  <label for="confirmPassword" class="block text-sm font-medium text-text-muted mb-1">Confirm New Password</label>
                  <input 
                    id="confirmPassword"
                    type="password" 
                    bind:value={confirmPassword}
                    class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors"
                  />
                </div>
                
                <button 
                  class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors shadow-[0_0_15px_rgba(102,126,234,0.3)] disabled:opacity-50 flex items-center gap-2"
                  onclick={savePassword}
                  disabled={saving || !currentPassword || !newPassword || !confirmPassword}
                >
                  {#if saving}
                    <div class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                    Saving...
                  {:else}
                    Update Password
                  {/if}
                </button>
              </div>
            </div>

            <!-- Upstream Proxy Section -->
            <div class="border-t border-border pt-6">
              <h2 class="text-lg font-medium text-white mb-1">Upstream Proxy</h2>
              <p class="text-sm text-text-muted mb-4">Configure HTTP/SOCKS5 proxy for browser automation (account login). Leave empty to connect directly.</p>
              
              <div class="space-y-4 max-w-md">
                <div>
                  <label for="upstreamProxy" class="block text-sm font-medium text-text-muted mb-1">Proxy URL</label>
                  <input 
                    id="upstreamProxy"
                    type="text" 
                    bind:value={upstreamProxy}
                    placeholder="http://113.160.132.26:8080 or socks5://user:pass@host:port"
                    class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors font-mono text-sm"
                  />
                  <p class="text-xs text-text-muted mt-1">Format: http://host:port or socks5://user:pass@host:port</p>
                </div>
                
                <button 
                  class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors shadow-[0_0_15px_rgba(102,126,234,0.3)] disabled:opacity-50 flex items-center gap-2"
                  onclick={saveUpstreamProxy}
                  disabled={saving}
                >
                  {#if saving}
                    <div class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                    Saving...
                  {:else}
                    Save Proxy
                  {/if}
                </button>
              </div>
            </div>
          </div>
        {:else if activeTab === 'network'}
          <div class="space-y-6 animate-in fade-in duration-300">
            <div>
              <h2 class="text-lg font-medium text-white mb-1">Network Interfaces</h2>
              <p class="text-sm text-text-muted mb-4">Configure which IP addresses and ports the proxy and dashboard bind to.</p>
              
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-6 max-w-2xl mb-6">
                <!-- Proxy Settings -->
                <div class="space-y-4 p-4 rounded-lg bg-bg-base border border-border">
                  <h3 class="font-medium text-white">Proxy Server</h3>
                  
                  <div>
                    <label for="proxyHost" class="block text-sm font-medium text-text-muted mb-1">Host</label>
                    <input 
                      id="proxyHost"
                      type="text" 
                      bind:value={networkSettings.proxy_host}
                      class="w-full bg-bg-sidebar border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors font-mono text-sm"
                    />
                  </div>
                  
                  <div>
                    <label for="proxyPort" class="block text-sm font-medium text-text-muted mb-1">Port</label>
                    <input 
                      id="proxyPort"
                      type="number" 
                      bind:value={networkSettings.proxy_port}
                      class="w-full bg-bg-sidebar border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors font-mono text-sm"
                    />
                  </div>
                </div>
                
                <!-- Dashboard Settings -->
                <div class="space-y-4 p-4 rounded-lg bg-bg-base border border-border">
                  <h3 class="font-medium text-white">Dashboard Server</h3>
                  
                  <div>
                    <label for="dashboardHost" class="block text-sm font-medium text-text-muted mb-1">Host</label>
                    <input 
                      id="dashboardHost"
                      type="text" 
                      bind:value={networkSettings.dashboard_host}
                      class="w-full bg-bg-sidebar border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors font-mono text-sm"
                    />
                  </div>
                  
                  <div>
                    <label for="dashboardPort" class="block text-sm font-medium text-text-muted mb-1">Port</label>
                    <input 
                      id="dashboardPort"
                      type="number" 
                      bind:value={networkSettings.dashboard_port}
                      class="w-full bg-bg-sidebar border border-border rounded-lg px-3 py-2 text-white focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors font-mono text-sm"
                    />
                  </div>
                </div>
              </div>
              
              <div class="space-y-4 max-w-2xl">
                <label class="flex items-center gap-3 cursor-pointer p-4 rounded-lg bg-bg-base border border-border hover:border-accent/50 transition-colors">
                  <div class="relative">
                    <input type="checkbox" class="sr-only" bind:checked={networkSettings.expose_to_network} />
                    <div class="block w-12 h-7 bg-bg-sidebar border border-border rounded-full transition-colors {networkSettings.expose_to_network ? 'bg-accent/20 border-accent' : ''}"></div>
                    <div class="dot absolute left-1 top-1 bg-text-muted w-5 h-5 rounded-full transition-transform {networkSettings.expose_to_network ? 'translate-x-5 bg-accent' : ''}"></div>
                  </div>
                  <div>
                    <div class="font-medium text-white">Expose to Network</div>
                    <div class="text-xs text-text-muted">Automatically listen on 0.0.0.0 instead of 127.0.0.1</div>
                  </div>
                </label>
                
                <div>
                  <label for="ipWhitelist" class="block text-sm font-medium text-text-muted mb-1">IP Whitelist (comma separated)</label>
                  <textarea 
                    id="ipWhitelist"
                    bind:value={ipWhitelistText}
                    rows="3"
                    placeholder="127.0.0.1, 192.168.1.0/24"
                    class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-white placeholder:text-border focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors font-mono text-sm resize-none"
                  ></textarea>
                  <p class="text-xs text-text-muted mt-1">Leave empty to allow all connections (not recommended for exposed servers)</p>
                </div>
                
                <button 
                  class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors shadow-[0_0_15px_rgba(102,126,234,0.3)] disabled:opacity-50 flex items-center gap-2 mt-4"
                  onclick={saveNetworkSettings}
                  disabled={saving}
                >
                  {#if saving}
                    <div class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                    Saving...
                  {:else}
                    Save Network Settings
                  {/if}
                </button>
              </div>
            </div>
          </div>
        {:else if activeTab === 'rtk'}
          <div class="space-y-6 animate-in fade-in duration-300">
            <div>
              <h2 class="text-lg font-medium text-white mb-1">RTK Compression</h2>
              <p class="text-sm text-text-muted mb-4">Compress recognized tool output before upstream requests. Unknown payloads pass through unchanged.</p>
              <div class="space-y-4 max-w-2xl">
                <label class="flex items-center gap-3 cursor-pointer p-4 rounded-lg bg-bg-base border border-border hover:border-accent/50 transition-colors">
                  <div class="relative">
                    <input type="checkbox" class="sr-only" bind:checked={rtkEnabled} />
                    <div class="block w-12 h-7 bg-bg-sidebar border border-border rounded-full transition-colors {rtkEnabled ? 'bg-accent/20 border-accent' : ''}"></div>
                    <div class="dot absolute left-1 top-1 bg-text-muted w-5 h-5 rounded-full transition-transform {rtkEnabled ? 'translate-x-5 bg-accent' : ''}"></div>
                  </div>
                  <div>
                    <div class="font-medium text-white">Enable RTK</div>
                    <div class="text-xs text-text-muted">Filters: git-diff, grep, ls, tree, find, log, smart-truncate.</div>
                  </div>
                </label>

                <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div class="rounded-lg bg-bg-base border border-border p-4">
                    <div class="text-xs uppercase tracking-wider text-text-muted mb-1">Status</div>
                    <div class="font-medium {rtkEnabled ? 'text-emerald-400' : 'text-yellow-400'}">{rtkEnabled ? 'Active' : 'Disabled'}</div>
                  </div>
                  <div class="rounded-lg bg-bg-base border border-border p-4">
                    <div class="text-xs uppercase tracking-wider text-text-muted mb-1">Mode</div>
                    <div class="font-medium text-white">Safe shrink-only</div>
                  </div>
                  <div class="rounded-lg bg-bg-base border border-border p-4">
                    <div class="text-xs uppercase tracking-wider text-text-muted mb-1">Unknown</div>
                    <div class="font-medium text-white">Passthrough</div>
                  </div>
                </div>

                <button class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 flex items-center gap-2" onclick={saveRTKSettings} disabled={saving}>
                  {saving ? 'Saving...' : 'Save RTK Settings'}
                </button>
              </div>
            </div>
          </div>
        {:else if activeTab === 'sync'}
          <div class="space-y-6 animate-in fade-in duration-300">
            <div>
              <h2 class="text-lg font-medium text-white mb-1">Cloud Sync</h2>
              <p class="text-sm text-text-muted mb-4">Optional personal config sync. Local operation never requires cloud sync.</p>
              <div class="space-y-4 max-w-2xl">
                <div class="rounded-lg border border-yellow-500/20 bg-yellow-500/10 p-4 text-sm text-yellow-100">Secrets are encrypted before sync. Browser cookies/session artifacts are not synced by default.</div>
                <div>
                  <label for="syncEndpoint" class="block text-sm font-medium text-text-muted mb-1">Sync Endpoint</label>
                  <input id="syncEndpoint" type="url" bind:value={syncEndpoint} placeholder="https://sync.example.com" class="w-full bg-bg-base border border-border rounded-lg px-3 py-2 text-white placeholder:text-border focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent transition-colors font-mono text-sm" />
                  <p class="text-xs text-text-muted mt-1">Leave empty to keep sync disabled.</p>
                </div>
                <button class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-50 flex items-center gap-2" onclick={saveSyncSettings} disabled={saving}>
                  {saving ? 'Saving...' : 'Save Sync Settings'}
                </button>
              </div>
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>
