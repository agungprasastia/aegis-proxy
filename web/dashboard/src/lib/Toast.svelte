<script>
    import { toast } from './stores/toast.js';
    import { fly, fade } from 'svelte/transition';

    // Using Svelte 5 runes for icon and color mapping
    let toastTypes = $state({
        success: {
            color: 'bg-green-500/10 border-green-500/20 text-green-400',
            icon: `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>`
        },
        error: {
            color: 'bg-red-500/10 border-red-500/20 text-red-400',
            icon: `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>`
        },
        warning: {
            color: 'bg-orange-500/10 border-orange-500/20 text-orange-400',
            icon: `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg>`
        },
        info: {
            color: 'bg-blue-500/10 border-blue-500/20 text-blue-400',
            icon: `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>`
        }
    });
</script>

<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
    {#each $toast as t (t.id)}
        <div 
            in:fly={{ x: 50, duration: 300 }}
            out:fade={{ duration: 200 }}
            class="pointer-events-auto flex items-start gap-3 p-4 rounded-lg border backdrop-blur-md shadow-lg transition-all duration-200 {toastTypes[t.type].color} bg-[#0d1117] min-w-[300px] max-w-md"
        >
            <div class="flex-shrink-0 mt-0.5">
                <!-- eslint-disable-next-line svelte/no-at-html-tags -->
                {@html toastTypes[t.type].icon}
            </div>
            
            <div class="flex-1 text-sm font-medium">
                {t.message}
            </div>

            <button 
                class="flex-shrink-0 opacity-70 hover:opacity-100 transition-opacity"
                onclick={() => toast.removeToast(t.id)}
                aria-label="Close"
            >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                </svg>
            </button>
        </div>
    {/each}
</div>
