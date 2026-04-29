import { writable } from 'svelte/store';
import { api } from '../api.js';

const TOKEN_KEY = 'session_token';

function createAuth() {
  const getInitialToken = () => {
    if (typeof localStorage !== 'undefined') {
      return localStorage.getItem(TOKEN_KEY);
    }
    return null;
  };

  const initialToken = getInitialToken();
  const { subscribe, set, update } = writable({
    isAuthenticated: !!initialToken,
    token: initialToken
  });

  return {
    subscribe,
    login: async (password) => {
      try {
        const response = await api.post('/api/auth/login', { password });
        if (response && response.token) {
          if (typeof localStorage !== 'undefined') {
            localStorage.setItem(TOKEN_KEY, response.token);
          }
          set({ isAuthenticated: true, token: response.token });
          return true;
        }
        throw new Error('Invalid response from server');
      } catch (error) {
        throw error;
      }
    },
    logout: () => {
      if (typeof localStorage !== 'undefined') {
        localStorage.removeItem(TOKEN_KEY);
      }
      set({ isAuthenticated: false, token: null });
    }
  };
}

export const auth = createAuth();
