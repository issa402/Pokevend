// ============================================================
// PokémonTool — Top Header Bar
// Displays page title, global search, and notification bell
// ============================================================

import { useState } from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { useNavigate, useLocation } from 'react-router-dom';
import { logout } from '../../store/index.js';
import { disconnectSSE } from '../../services/sse.js';
import { Search, Bell, LogOut, User } from 'lucide-react';

// Map route paths to human-readable titles
const PAGE_TITLES = {
  '/':          'Dashboard',
  '/watchlist': 'My Watchlist',
  '/trends':    'Market Trends',
  '/inventory': 'Inventory',
  '/shows':     'Upcoming Shows',
  '/alerts':    'Alerts',
  '/settings':  'Settings',
};

export default function Header() {
  const dispatch    = useDispatch();
  const navigate    = useNavigate();
  const location    = useLocation();
  const unreadCount = useSelector(s => s.alerts.unreadCount);
  const user        = useSelector(s => s.auth.user);
  const [search, setSearch] = useState('');

  function handleLogout() {
    disconnectSSE();
    dispatch(logout());
    navigate('/login');
  }

  function handleSearch(e) {
    e.preventDefault();
    if (search.trim()) {
      navigate(`/trends?q=${encodeURIComponent(search.trim())}`);
    }
  }

  const title = PAGE_TITLES[location.pathname] || 'PokémonTool';

  return (
    <header style={{
      gridColumn:  '2',
      gridRow:     '1',
      background:  'var(--color-bg-surface)',
      borderBottom:'1px solid var(--color-border)',
      display:     'flex',
      alignItems:  'center',
      gap:         20,
      padding:     '0 24px',
      height:      'var(--header-height)',
      position:    'sticky',
      top:         0,
      zIndex:      100,
    }}>
      {/* Page title */}
      <h1 style={{ fontWeight: 700, fontSize: '1.125rem', flex: 'none' }}>{title}</h1>

      {/* Global card search */}
      <form onSubmit={handleSearch} style={{ flex: 1, maxWidth: 420, position: 'relative' }}>
        <Search size={15} style={{
          position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)',
          color: 'var(--color-text-muted)',
        }} />
        <input
          className="input"
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="Search any Pokémon card..."
          style={{ paddingLeft: 36, height: 38 }}
        />
      </form>

      {/* Notification bell */}
      <button
        className="btn btn-secondary btn-icon"
        onClick={() => navigate('/alerts')}
        title="View alerts"
        style={{ position: 'relative' }}
      >
        <Bell size={18} />
        {unreadCount > 0 && (
          <span style={{
            position: 'absolute', top: -6, right: -6,
            background: 'var(--color-accent-falling)',
            color: '#fff',
            borderRadius: 100, padding: '1px 6px',
            fontSize: '0.65rem', fontWeight: 700,
            minWidth: 18, textAlign: 'center',
          }}>
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {/* User avatar + logout */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{
          width: 32, height: 32,
          background: 'var(--gradient-primary)',
          borderRadius: '50%',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <User size={16} color="#fff" />
        </div>
        <span style={{ fontSize: '0.8125rem', color: 'var(--color-text-secondary)' }}>
          {user?.displayName || user?.email?.split('@')[0] || 'Vendor'}
        </span>
        <button className="btn btn-secondary btn-icon" onClick={handleLogout} title="Logout">
          <LogOut size={16} />
        </button>
      </div>
    </header>
  );
}
