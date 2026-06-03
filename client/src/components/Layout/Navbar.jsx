// ============================================================
// PokémonTool — Sidebar Navigation
// ============================================================

import { NavLink } from 'react-router-dom';
import { useSelector } from 'react-redux';
import {
  LayoutDashboard, Star, TrendingUp, Package,
  MapPin, Bell, Settings, Zap, Trophy,
} from 'lucide-react';

// Navigation items — icon, label, and path
const NAV_ITEMS = [
  { to: '/',          icon: LayoutDashboard, label: 'Dashboard'  },
  { to: '/watchlist', icon: Star,            label: 'Watchlist'  },
  { to: '/trends',    icon: TrendingUp,      label: 'Trends'     },
  { to: '/inventory', icon: Package,         label: 'Inventory'  },
  { to: '/finder', icon: Trophy, label: 'Finder' },
  { to: '/shows',     icon: MapPin,          label: 'Shows'      },
  { to: '/alerts',    icon: Bell,            label: 'Alerts'     },
  { to: '/settings',  icon: Settings,        label: 'Settings'   },
];

export default function Navbar() {
  const unreadCount = useSelector(s => s.alerts.unreadCount);

  return (
    <nav style={{
      gridRow:    '1 / -1',     // Spans both header and content rows
      gridColumn: '1',
      background: 'var(--color-bg-surface)',
      borderRight:'1px solid var(--color-border)',
      display:    'flex',
      flexDirection: 'column',
      padding:    '0 0 24px 0',
      overflowY:  'auto',
    }}>
      {/* Logo / brand */}
      <div style={{
        padding: '20px 20px 24px',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        borderBottom: '1px solid var(--color-border)',
        marginBottom: 8,
      }}>
        <div style={{
          width: 36, height: 36,
          background: 'var(--gradient-primary)',
          borderRadius: 10,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          boxShadow: 'var(--shadow-glow)',
        }}>
          <Zap size={20} color="#fff" />
        </div>
        <div>
          <div style={{ fontWeight: 800, fontSize: '0.95rem', letterSpacing: '-0.3px' }}>PokémonTool</div>
          <div style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)' }}>Vendor Intelligence</div>
        </div>
      </div>

      {/* Navigation links */}
      <div style={{ flex: 1, padding: '8px 12px', display: 'flex', flexDirection: 'column', gap: 2 }}>
        {NAV_ITEMS.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}  // Exact match for dashboard (prevents always being active)
            style={({ isActive }) => ({
              display:      'flex',
              alignItems:   'center',
              gap:          12,
              padding:      '10px 14px',
              borderRadius: 'var(--radius-md)',
              color:        isActive ? '#fff' : 'var(--color-text-secondary)',
              background:   isActive ? 'var(--gradient-primary)' : 'transparent',
              boxShadow:    isActive ? 'var(--shadow-glow)' : 'none',
              fontWeight:   isActive ? 600 : 400,
              fontSize:     '0.875rem',
              transition:   'all var(--transition-fast)',
              position:     'relative',
            })}
          >
            <Icon size={18} />
            {label}
            {/* Unread badge on Alerts nav item */}
            {label === 'Alerts' && unreadCount > 0 && (
              <span style={{
                marginLeft: 'auto',
                background: 'var(--color-accent-falling)',
                color: '#fff',
                borderRadius: 100,
                padding: '1px 7px',
                fontSize: '0.7rem',
                fontWeight: 700,
              }}>
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </NavLink>
        ))}
      </div>

      {/* Live indicator at bottom */}
      <div style={{
        margin: '0 12px',
        padding: '10px 14px',
        background: 'rgba(16,185,129,0.08)',
        borderRadius: 'var(--radius-md)',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        border: '1px solid rgba(16,185,129,0.2)',
      }}>
        <div className="live-dot" />
        <div>
          <div style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--color-accent-rising)' }}>Live Monitoring</div>
          <div style={{ fontSize: '0.65rem', color: 'var(--color-text-muted)' }}>Market data streaming</div>
        </div>
      </div>
    </nav>
  );
}
