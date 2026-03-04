// ============================================================
// PokémonTool — Alerts Page
// Full history of all triggered price and trend alerts
// ============================================================

import { useEffect } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { Bell, CheckCheck, ExternalLink, TrendingUp, TrendingDown, Zap } from 'lucide-react';
import api from '../services/api.js';
import { setAlerts, markAlertsRead } from '../store/index.js';

// Map alert type to icon + color
const ALERT_META = {
  PRICE_DROP:  { icon: TrendingDown, color: 'var(--color-accent-rising)',   label: 'Price Drop'   },
  PRICE_SPIKE: { icon: TrendingUp,   color: 'var(--color-accent-falling)',  label: 'Price Spike'  },
  TREND_CHANGE:{ icon: TrendingUp,   color: 'var(--color-accent-secondary)',label: 'Trend Change' },
  DEAL_FOUND:  { icon: Zap,          color: 'var(--color-accent-gold)',     label: 'Deal Found'   },
  DEAL_OF_DAY: { icon: Zap,          color: 'var(--color-accent-gold)',     label: 'Deal of Day'  },
};

export default function AlertsPage() {
  const dispatch    = useDispatch();
  const { items, unreadCount } = useSelector(s => s.alerts);

  useEffect(() => {
    api.get('/alerts?limit=100').then(r => {
      dispatch(setAlerts({ alerts: r.data.alerts, unreadCount: r.data.unreadCount }));
    });
  }, [dispatch]);

  async function handleMarkAllRead() {
    await api.put('/alerts/read-all');
    dispatch(markAlertsRead());
  }

  return (
    <div>
      <div className="section-header">
        <div className="section-title"><Bell size={18} className="icon" /> Alerts ({items.length}){unreadCount > 0 && <span className="badge badge-falling" style={{ marginLeft:8 }}>{unreadCount} unread</span>}</div>
        {unreadCount > 0 && (
          <button className="btn btn-secondary btn-sm" onClick={handleMarkAllRead}><CheckCheck size={14} /> Mark all read</button>
        )}
      </div>

      {items.length > 0 ? (
        <div className="glass-card" style={{ overflow:'hidden' }}>
          {items.map((alert, i) => {
            const meta = ALERT_META[alert.alert_type || alert.alertType] || ALERT_META.PRICE_DROP;
            const Icon = meta.icon;
            return (
              <div key={alert.id} className={`alert-item ${alert.alert_type || alert.alertType} ${!(alert.is_read || alert.isRead) ? 'unread' : ''}`} style={{ borderBottom: i < items.length - 1 ? '1px solid var(--color-border)' : 'none' }}>
                <div style={{ width:36, height:36, background:`${meta.color}18`, borderRadius:10, display:'flex', alignItems:'center', justifyContent:'center', flexShrink:0 }}>
                  <Icon size={16} color={meta.color} />
                </div>
                <div style={{ flex:1 }}>
                  <div style={{ display:'flex', alignItems:'center', gap:8, marginBottom:4 }}>
                    <span className="badge" style={{ background:`${meta.color}18`, color:meta.color }}>{meta.label}</span>
                    <span style={{ fontWeight:600, fontSize:'0.875rem' }}>{alert.card_name || alert.cardName}</span>
                    {!(alert.is_read || alert.isRead) && <span style={{ width:8, height:8, background:'var(--color-accent-primary)', borderRadius:'50%', display:'inline-block' }} />}
                  </div>
                  <div style={{ fontSize:'0.8125rem', color:'var(--color-text-secondary)', marginBottom:4 }}>{alert.message}</div>
                  <div style={{ display:'flex', alignItems:'center', gap:16 }}>
                    <span style={{ fontSize:'0.75rem', color:'var(--color-text-muted)' }}>{new Date(alert.created_at || alert.createdAt).toLocaleString()}</span>
                    {alert.marketplace && <span className={`badge badge-ebay`} style={{ fontSize:'0.65rem' }}>{alert.marketplace}</span>}
                    {(alert.listing_url || alert.listingUrl) && (
                      <a href={alert.listing_url || alert.listingUrl} target="_blank" rel="noreferrer" style={{ fontSize:'0.75rem', display:'flex', alignItems:'center', gap:4 }}>
                        View <ExternalLink size={11} />
                      </a>
                    )}
                  </div>
                </div>
                {(alert.price) && (
                  <div className="price-tag">${parseFloat(alert.price).toFixed(2)}</div>
                )}
              </div>
            );
          })}
        </div>
      ) : (
        <div className="glass-card empty-state" style={{ padding:60 }}>
          <Bell size={48} />
          <h3 style={{ fontWeight:700 }}>No alerts yet</h3>
          <p>Add cards to your watchlist to start receiving real-time price alerts</p>
        </div>
      )}
    </div>
  );
}
