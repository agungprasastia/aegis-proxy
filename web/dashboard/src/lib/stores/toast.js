import { writable } from 'svelte/store';

function createToastStore() {
    const { subscribe, update } = writable([]);

    function addToast({ type = 'info', message, duration = 4000 }) {
        const id = Date.now() + Math.random();
        const t = { id, type, message, duration };
        
        update(toasts => [...toasts, t]);

        if (duration) {
            setTimeout(() => {
                update(toasts => toasts.filter(x => x.id !== id));
            }, duration);
        }
    }

    return {
        subscribe,
        addToast,
        removeToast: (id) => {
            update(toasts => toasts.filter(t => t.id !== id));
        },
        // Shortcut methods used by pages
        success: (message) => addToast({ type: 'success', message }),
        error: (message) => addToast({ type: 'error', message }),
        warning: (message) => addToast({ type: 'warning', message }),
        info: (message) => addToast({ type: 'info', message }),
    };
}

export const toast = createToastStore();
