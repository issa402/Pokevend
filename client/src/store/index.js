// ============================================================
// PokémonTool — Redux Store
// Central state management for the entire application.
// Slices: auth, watchlist, alerts, trends, inventory
// ============================================================

import { configureStore, createSlice } from '@reduxjs/toolkit';

// ---- Auth Slice ----
// Tracks login state and user profile
const authSlice = createSlice({
  name: 'auth',
  initialState: {
    user:  JSON.parse(localStorage.getItem('pt_user') || 'null'),
    token: localStorage.getItem('pt_token') || null,
    isAuthenticated: !!localStorage.getItem('pt_token'),
  },
  reducers: {
    setCredentials: (state, action) => {
      const { user, token } = action.payload;
      state.user            = user;
      state.token           = token;
      state.isAuthenticated = true;
      // Persist to localStorage so the user stays logged in on refresh
      localStorage.setItem('pt_token', token);
      localStorage.setItem('pt_user',  JSON.stringify(user));
    },
    logout: (state) => {
      state.user            = null;
      state.token           = null;
      state.isAuthenticated = false;
      localStorage.removeItem('pt_token');
      localStorage.removeItem('pt_user');
    },
  },
});

// ---- Alerts Slice ----
// Stores incoming real-time and historical alerts
const alertsSlice = createSlice({
  name: 'alerts',
  initialState: { items: [], unreadCount: 0 },
  reducers: {
    setAlerts: (state, action) => {
      state.items       = action.payload.alerts;
      state.unreadCount = action.payload.unreadCount;
    },
    // Push a single new alert (received via SSE)
    addAlert: (state, action) => {
      state.items.unshift(action.payload); // Add to front (newest first)
      state.unreadCount += 1;
    },
    markAllRead: (state) => {
      state.items       = state.items.map(a => ({ ...a, isRead: true }));
      state.unreadCount = 0;
    },
  },
});

// ---- Watchlist Slice ----
const watchlistSlice = createSlice({
  name: 'watchlist',
  initialState: { items: [] },
  reducers: {
    setWatchlist: (state, action) => { state.items = action.payload; },
    addWatchlistItem: (state, action) => { state.items.unshift(action.payload); },
    removeWatchlistItem: (state, action) => {
      state.items = state.items.filter(i => i.id !== action.payload);
    },
  },
});

// ---- Trends Slice ----
const trendsSlice = createSlice({
  name: 'trends',
  initialState: { rising: [], falling: [], updatedAt: null },
  reducers: {
    setTrends: (state, action) => {
      state.rising    = action.payload.rising;
      state.falling   = action.payload.falling;
      state.updatedAt = action.payload.updatedAt;
    },
  },
});

// ---- Inventory Slice ----
const inventorySlice = createSlice({
  name: 'inventory',
  initialState: { items: [], total: 0 },
  reducers: {
    setInventory: (state, action) => {
      state.items = action.payload.inventory;
      state.total = action.payload.total;
    },
    removeInventoryItem: (state, action) => {
      state.items = state.items.filter(i => i.id !== action.payload);
      state.total = Math.max(0, state.total - 1);
    },
  },
});

// Export all action creators for use in components
export const { setCredentials, logout }                            = authSlice.actions;
export const { setAlerts, addAlert, markAllRead: markAlertsRead }  = alertsSlice.actions;
export const { setWatchlist, addWatchlistItem, removeWatchlistItem }= watchlistSlice.actions;
export const { setTrends }                                         = trendsSlice.actions;
export const { setInventory, removeInventoryItem }                 = inventorySlice.actions;

// Assemble the store from all slices
export const store = configureStore({
  reducer: {
    auth:      authSlice.reducer,
    alerts:    alertsSlice.reducer,
    watchlist: watchlistSlice.reducer,
    trends:    trendsSlice.reducer,
    inventory: inventorySlice.reducer,
  },
});
