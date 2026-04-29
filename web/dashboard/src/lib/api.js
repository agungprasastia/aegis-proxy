// When embedded in Go binary, use relative URL (same origin)
// When running dev server (Vite), use absolute URL to backend
const BASE_URL = typeof window !== 'undefined' && window.location.port === '5173' 
  ? 'http://localhost:3131' 
  : '';
const TOKEN_KEY = 'session_token';

function getToken() {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(TOKEN_KEY);
}

function redirectToLogin() {
  if (typeof localStorage !== 'undefined') {
    localStorage.removeItem(TOKEN_KEY);
  }
  // Only redirect if not already on login (prevent infinite loop)
  if (typeof window !== 'undefined' && !window.__aegis_redirecting) {
    window.__aegis_redirecting = true;
    window.location.reload();
  }
}

function buildHeaders(hasBody = false) {
  const headers = {};
  const token = getToken();

  if (token) headers.Authorization = `Bearer ${token}`;
  if (hasBody) headers['Content-Type'] = 'application/json';

  return headers;
}

function parseResponse(response, path) {
  if (response.status === 401 && !path?.includes('/api/auth/login')) {
    redirectToLogin();
    throw new Error('Unauthorized');
  }

  return response.text().then((text) => {
    const data = text ? JSON.parse(text) : null;

    if (!response.ok) {
      const message = data?.error || data?.message || response.statusText || 'Request failed';
      throw new Error(message);
    }

    return data;
  }).catch((error) => {
    if (error instanceof SyntaxError) {
      if (!response.ok) throw new Error(response.statusText || 'Request failed');
      return null;
    }
    throw error;
  });
}

function request(path, options = {}) {
  return fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      ...buildHeaders(Boolean(options.body)),
      ...(options.headers || {}),
    },
  }).then(res => parseResponse(res, path));
}

export const api = {
  get(path) {
    return request(path, { method: 'GET' });
  },
  post(path, body) {
    return request(path, { method: 'POST', body: JSON.stringify(body) });
  },
  put(path, body) {
    return request(path, { method: 'PUT', body: JSON.stringify(body) });
  },
  delete(path, body) {
    return request(path, { method: 'DELETE', body: body === undefined ? undefined : JSON.stringify(body) });
  },
};
