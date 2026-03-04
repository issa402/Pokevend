// ============================================================
// PokémonTool — Axios API Client
// Preconfigured instance that automatically injects Auth headers.
// ============================================================

import axios from 'axios';

// Base URL points to our Node.js API gateway.
// In dev, Vite proxies /api → localhost:3001 (see vite.config.js)
const api = axios.create({
  baseURL: '/api',
  timeout: 10000, // 10 second timeout
  headers: { 'Content-Type': 'application/json' },
});

// ---- Request Interceptor ----
// Automatically injects the JWT Bearer token on every request
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('pt_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// ---- Response Interceptor ----
// Handles 401 responses (expired token) by logging the user out
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Token expired — clear storage and redirect to login
      localStorage.removeItem('pt_token');
      localStorage.removeItem('pt_user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export default api;
