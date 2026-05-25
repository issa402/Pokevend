// ============================================================
// PokémonTool — Dashboard Page
// Main hub showing: stats, top movers, deal of day, recent alerts
// ============================================================

import { useEffect, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { Link } from 'react-router-dom';
import { TrendingUp, TrendingDown, Bell, Package, Star, Zap, ArrowRight, ExternalLink } from 'lucide-react';
import api from '../services/api.js';
import { setAlerts } from '../store/index.js';

export default function DashboardPage() {
  const dispatch    = useDispatch();
  const alerts      = useSelector(s => Array.isArray(s.alerts.items) ? s.alerts.items : []);
  const watchlist   = useSelector(s => Array.isArray(s.watchlist.items) ? s.watchlist.items : []);
  const [deals, setDeals]   = useState([]);
  const [trends, setTrends] = useState({ rising: [], falling: [] });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function loadDashboard() {
      try {
        // Load all dashboard data in parallel
        const [alertsRes, trendsRes, dealsRes] = await Promise.allSettled([
          api.get('/alerts?limit=5'),
          api.get('/cards/trending'),
          api.get('/deals/today'),
        ]);
        if (alertsRes.status === 'fulfilled') {
          const data = alertsRes.value.data;
          dispatch(setAlerts({
            alerts: Array.isArray(data) ? data : data.alerts,
            unreadCount: Array.isArray(data) ? data.filter(a => !(a.is_read || a.isRead)).length : data.unreadCount,
          }));
        }
        if (trendsRes.status === 'fulfilled') setTrends(trendsRes.value.data);
        if (dealsRes.status === 'fulfilled')  setDeals(dealsRes.value.data.deals || []);
      } catch (e) {
        console.error('Dashboard load error:', e);
      } finally {
        setLoading(false);
      }
    }
    loadDashboard();
  }, [dispatch]);

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 60 }}><div className="spinner" /></div>;

  const topRising  = trends.rising?.slice(0, 3)  || [];
  const topFalling = trends.falling?.slice(0, 3) || [];
  const recentAlerts = alerts.slice(0, 5);
  const topDeal    = deals[0];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>

      {/* ---- Stat tiles ---- */}
      <div className="grid-4">
        {[
          { label: 'Watchlist Items',    value: watchlist.length,       icon: Star,       color: 'var(--color-accent-primary)'   },
          { label: 'Unread Alerts',      value: alerts.filter(a => !(a.is_read || a.isRead)).length, icon: Bell, color: 'var(--color-accent-warning)' },
          { label: 'Rising Cards Today', value: trends.rising?.length || 0, icon: TrendingUp,  color: 'var(--color-accent-rising)'  },
          { label: 'Deals Found Today',  value: deals.length,           icon: Zap,        color: 'var(--color-accent-gold)'      },
        ].map(({ label, value, icon: Icon, color }) => (
          <div key={label} className="stat-tile glass-card" style={{ padding: '20px 24px' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <span className="stat-label" style={{ marginTop: 0 }}>{label}</span>
              <div style={{ width: 36, height: 36, borderRadius: 10, background: `${color}22`, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Icon size={18} color={color} />
              </div>
            </div>
            <div className="stat-value">{value}</div>
          </div>
        ))}
      </div>

      <div className="grid-2">
        {/* ---- Deal of the Day ---- */}
        <div className="glass-card" style={{ padding: 24 }}>
          <div className="section-header">
            <div className="section-title"><Zap size={18} className="icon" /> Deal of the Day</div>
            <Link to="/trends" style={{ fontSize: '0.8125rem', color: 'var(--color-text-muted)', display: 'flex', alignItems: 'center', gap: 4 }}>View all <ArrowRight size={14} /></Link>
          </div>
          {topDeal ? (
            <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
              <div style={{ width: 80, height: 112, background: 'var(--color-bg-elevated)', borderRadius: 8, flexShrink: 0, overflow: 'hidden' }}>
                {topDeal.imageUrl && <img src={topDeal.imageUrl} alt={topDeal.cardName} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />}
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 700, marginBottom: 6 }}>{topDeal.cardName}</div>
                <div className="badge badge-gold" style={{ marginBottom: 10 }}>🔥 {topDeal.savingsPct?.toFixed(1)}% below market</div>
                <div style={{ display: 'flex', gap: 16, marginBottom: 10 }}>
                  <div>
                    <div style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)' }}>Listed Price</div>
                    <div className="price-tag">${topDeal.bestPrice?.toFixed(2)}</div>
                  </div>
                  <div>
                    <div style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)' }}>Market Avg</div>
                    <div style={{ fontWeight: 600, color: 'var(--color-text-secondary)', textDecoration: 'line-through' }}>${topDeal.marketPrice?.toFixed(2)}</div>
                  </div>
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', marginBottom: 12 }}>{topDeal.reason}</div>
                <a href={topDeal.listingUrl} target="_blank" rel="noreferrer" className="btn btn-primary btn-sm">
                  <ExternalLink size={13} /> View Listing
                </a>
              </div>
            </div>
          ) : (
            <div className="empty-state">
              <Zap size={40} />
              <p>Deals are computed at 6 AM daily. Check back soon!</p>
            </div>
          )}
        </div>

        {/* ---- Recent Alerts ---- */}
        <div className="glass-card" style={{ padding: 24 }}>
          <div className="section-header">
            <div className="section-title"><Bell size={18} className="icon" /> Recent Alerts</div>
            <Link to="/alerts" style={{ fontSize: '0.8125rem', color: 'var(--color-text-muted)', display: 'flex', alignItems: 'center', gap: 4 }}>View all <ArrowRight size={14} /></Link>
          </div>
          {recentAlerts.length > 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              {recentAlerts.map(alert => (
                <div key={alert.id} className={`alert-item ${alert.alertType} ${!alert.isRead ? 'unread' : ''}`}>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontWeight: 500, fontSize: '0.8125rem' }}>{alert.cardName || 'Market Update'}</div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', marginTop: 2 }}>{alert.message}</div>
                  </div>
                  {alert.price && <div className="price-tag" style={{ fontSize: '0.875rem' }}>${Number(alert.price).toFixed(2)}</div>}
                </div>
              ))}
            </div>
          ) : (
            <div className="empty-state"><Bell size={36} /><p>No alerts yet. Add cards to your watchlist to get started!</p></div>
          )}
        </div>
      </div>

      {/* ---- Top Movers ---- */}
      <div className="grid-2">
        <div className="glass-card" style={{ padding: 24 }}>
          <div className="section-title" style={{ marginBottom: 16 }}><TrendingUp size={18} color="var(--color-accent-rising)" /> 🚀 Rising Cards</div>
          {topRising.length > 0 ? topRising.map(card => (
            <div key={card.cardId || card._id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 0', borderBottom: '1px solid var(--color-border)' }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 600, fontSize: '0.875rem' }}>{card.name}</div>
                <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>{card.setName}</div>
              </div>
              <div className="badge badge-rising">+{card.pctChange7d?.toFixed(1)}%</div>
            </div>
          )) : <div className="empty-state" style={{ paddingTop: 20, paddingBottom: 20 }}><p>Trend data calculated hourly</p></div>}
        </div>
        <div className="glass-card" style={{ padding: 24 }}>
          <div className="section-title" style={{ marginBottom: 16 }}><TrendingDown size={18} color="var(--color-accent-falling)" /> 📉 Falling Cards</div>
          {topFalling.length > 0 ? topFalling.map(card => (
            <div key={card.cardId || card._id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 0', borderBottom: '1px solid var(--color-border)' }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 600, fontSize: '0.875rem' }}>{card.name}</div>
                <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>{card.setName}</div>
              </div>
              <div className="badge badge-falling">{card.pctChange7d?.toFixed(1)}%</div>
            </div>
          )) : <div className="empty-state" style={{ paddingTop: 20, paddingBottom: 20 }}><p>Trend data calculated hourly</p></div>}
        </div>
      </div>
    </div>
  );
}
