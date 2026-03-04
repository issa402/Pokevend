// ============================================================
// PokémonTool — Trends Page
// Shows rising/falling card analytics with price history charts
// ============================================================

import { useEffect, useState, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { TrendingUp, TrendingDown, Search } from 'lucide-react';
import {
  Chart as ChartJS, LineElement, PointElement, LinearScale,
  CategoryScale, Tooltip, Legend, Filler
} from 'chart.js';
import { Line } from 'react-chartjs-2';
import api from '../services/api.js';

ChartJS.register(LineElement, PointElement, LinearScale, CategoryScale, Tooltip, Legend, Filler);

export default function TrendsPage() {
  const [searchParams] = useSearchParams();
  const [rising,  setRising]   = useState([]);
  const [falling, setFalling]  = useState([]);
  const [query,   setQuery]    = useState(searchParams.get('q') || '');
  const [results, setResults]  = useState([]);
  const [loading, setLoading]  = useState(true);
  const [selected, setSelected]= useState(null);  // Card selected for price history
  const [history, setHistory]  = useState([]);

  useEffect(() => {
    api.get('/cards/trending').then(r => {
      setRising(r.data.rising   || []);
      setFalling(r.data.falling || []);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  // If a search query was passed (from header), run search immediately
  useEffect(() => {
    if (query) handleSearch();
  }, []);

  async function handleSearch(e) {
    if (e) e.preventDefault();
    if (!query.trim()) return;
    const { data } = await api.get(`/cards/search?q=${encodeURIComponent(query)}`);
    setResults(data.cards || []);
  }

  async function loadHistory(card) {
    setSelected(card);
    try {
      const { data } = await api.get(`/cards/${card.cardId}/history`);
      setHistory(data.card?.priceHistory || []);
    } catch { setHistory([]); }
  }

  // Chart.js config for the price history line chart
  const chartData = {
    labels: history.map(h => new Date(h.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })),
    datasets: [{
      label: 'Avg Price ($)',
      data:  history.map(h => h.avgPrice),
      borderColor: 'var(--color-accent-primary)',
      backgroundColor: 'rgba(79,142,247,0.08)',
      borderWidth: 2,
      pointRadius: 3,
      tension: 0.4,
      fill: true,
    }],
  };

  const chartOptions = {
    responsive: true,
    plugins: { legend: { display: false }, tooltip: { callbacks: { label: ctx => ` $${ctx.raw?.toFixed(2)}` } } },
    scales: {
      x: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: 'var(--color-text-muted)', font: { size: 11 } } },
      y: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: 'var(--color-text-muted)', callback: v => `$${v}` } },
    },
  };

  if (loading) return <div style={{ display:'flex', justifyContent:'center', paddingTop:60 }}><div className="spinner" /></div>;

  return (
    <div style={{ display:'flex', flexDirection:'column', gap:24 }}>
      {/* Search */}
      <div className="glass-card" style={{ padding:20 }}>
        <form onSubmit={handleSearch} style={{ display:'flex', gap:12 }}>
          <input className="input" value={query} onChange={e => setQuery(e.target.value)} placeholder="Search any card for price history and trends..." style={{ flex:1 }} />
          <button className="btn btn-primary" type="submit"><Search size={16} /> Search</button>
        </form>
        {results.length > 0 && (
          <div style={{ marginTop:16, display:'flex', flexDirection:'column', gap:6 }}>
            {results.map(card => (
              <div key={card.cardId} onClick={() => loadHistory(card)} style={{ display:'flex', alignItems:'center', gap:12, padding:'10px 14px', background:'var(--color-bg-elevated)', borderRadius:'var(--radius-md)', cursor:'pointer', border:`1px solid ${selected?.cardId === card.cardId ? 'var(--color-accent-primary)' : 'transparent'}` }}>
                <div style={{ flex:1 }}>
                  <div style={{ fontWeight:600, fontSize:'0.875rem' }}>{card.name}</div>
                  <div style={{ fontSize:'0.75rem', color:'var(--color-text-muted)' }}>{card.setName}</div>
                </div>
                <span className={`badge badge-${(card.trendLabel||'stable').toLowerCase()}`}>{card.trendLabel || 'STABLE'}</span>
                {card.prices?.ebay && <strong style={{ color:'var(--color-accent-gold)' }}>${card.prices.ebay.toFixed(2)}</strong>}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Price History Chart */}
      {selected && (
        <div className="glass-card" style={{ padding:24 }}>
          <div style={{ fontWeight:700, marginBottom:16 }}>📈 Price History — {selected.name}</div>
          {history.length > 0
            ? <Line data={chartData} options={chartOptions} height={80} />
            : <div className="empty-state" style={{ padding:40 }}><p>No price history available yet for this card.</p></div>
          }
        </div>
      )}

      {/* Rising / Falling grids */}
      <div className="grid-2">
        <div className="glass-card" style={{ padding:24 }}>
          <div className="section-title" style={{ marginBottom:16 }}><TrendingUp size={18} color="var(--color-accent-rising)" /> 🚀 Rising Cards</div>
          {rising.length > 0 ? rising.map(card => (
            <div key={card.cardId} onClick={() => loadHistory(card)} style={{ display:'flex', alignItems:'center', gap:12, padding:'10px 0', borderBottom:'1px solid var(--color-border)', cursor:'pointer' }}>
              <div style={{ flex:1 }}>
                <div style={{ fontWeight:600, fontSize:'0.875rem' }}>{card.name}</div>
                <div style={{ fontSize:'0.75rem', color:'var(--color-text-muted)' }}>{card.setName}</div>
              </div>
              <div>
                <div className="badge badge-rising" style={{ marginBottom:4 }}>+{card.pctChange7d?.toFixed(1)}% 7d</div>
                {card.prices?.tcgplayer && <div style={{ textAlign:'right', fontSize:'0.8125rem', fontWeight:600, color:'var(--color-accent-gold)' }}>${card.prices.tcgplayer.toFixed(2)}</div>}
              </div>
            </div>
          )) : <div className="empty-state" style={{ padding:30 }}><p>Trend data updates hourly</p></div>}
        </div>
        <div className="glass-card" style={{ padding:24 }}>
          <div className="section-title" style={{ marginBottom:16 }}><TrendingDown size={18} color="var(--color-accent-falling)" /> 📉 Falling Cards</div>
          {falling.length > 0 ? falling.map(card => (
            <div key={card.cardId} onClick={() => loadHistory(card)} style={{ display:'flex', alignItems:'center', gap:12, padding:'10px 0', borderBottom:'1px solid var(--color-border)', cursor:'pointer' }}>
              <div style={{ flex:1 }}>
                <div style={{ fontWeight:600, fontSize:'0.875rem' }}>{card.name}</div>
                <div style={{ fontSize:'0.75rem', color:'var(--color-text-muted)' }}>{card.setName}</div>
              </div>
              <div>
                <div className="badge badge-falling">{card.pctChange7d?.toFixed(1)}% 7d</div>
                {card.prices?.tcgplayer && <div style={{ textAlign:'right', fontSize:'0.8125rem', fontWeight:600, color:'var(--color-accent-gold)' }}>${card.prices.tcgplayer.toFixed(2)}</div>}
              </div>
            </div>
          )) : <div className="empty-state" style={{ padding:30 }}><p>Trend data updates hourly</p></div>}
        </div>
      </div>
    </div>
  );
}
