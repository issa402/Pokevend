// ============================================================
// PokémonTool — App.jsx
// Top-level router. Protects dashboard routes behind auth.
// Starts the SSE stream after login.
// ============================================================

import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { useSelector } from 'react-redux';

import { connectSSE, disconnectSSE } from './services/sse.js';

import Navbar       from './components/Layout/Navbar.jsx';
import Header       from './components/Layout/Header.jsx';
import DashboardPage  from './pages/DashboardPage.jsx';
import WatchlistPage  from './pages/WatchlistPage.jsx';
import TrendsPage     from './pages/TrendsPage.jsx';
import InventoryPage  from './pages/InventoryPage.jsx';
import ShowsPage      from './pages/ShowsPage.jsx';
import AlertsPage     from './pages/AlertsPage.jsx';
import SettingsPage   from './pages/SettingsPage.jsx';
import SlabOpportunitiesPage from './pages/SlabOpportunitiesPage.jsx';
import LoginPage      from './pages/LoginPage.jsx';

// ---- Protected Layout ----
// Renders the sidebar + header + main content area.
// Redirects to /login if the user is not authenticated.
function ProtectedLayout() {
  const isAuthenticated = useSelector(s => s.auth.isAuthenticated);

  useEffect(() => {
    if (isAuthenticated) {
      connectSSE(); // Open the real-time event stream
    }
    return () => disconnectSSE(); // Clean up stream on unmount
  }, [isAuthenticated]);

  if (!isAuthenticated) return <Navigate to="/login" replace />;

  return (
    <div className="app-layout">
      <Navbar />
      <Header />
      <main className="main-content">
        <div className="page-enter">
          <Outlet /> {/* Active page renders here */}
        </div>
      </main>
    </div>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />

        {/* Protected dashboard routes */}
        <Route element={<ProtectedLayout />}>
          <Route path="/"          element={<DashboardPage />} />
          <Route path="/watchlist" element={<WatchlistPage />} />
          <Route path="/trends"    element={<TrendsPage />} />
          <Route path="/inventory" element={<InventoryPage />} />
          <Route path="/finder" element={<SlabOpportunitiesPage />} />
          <Route path="/slab-opportunities" element={<Navigate to="/finder" replace />} />
          <Route path="/shows"     element={<ShowsPage />} />
          <Route path="/alerts"    element={<AlertsPage />} />
          <Route path="/settings"  element={<SettingsPage />} />
        </Route>

        {/* Catch-all: redirect unknown routes to dashboard */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
