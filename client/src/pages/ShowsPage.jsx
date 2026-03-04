// ============================================================
// PokémonTool — Shows Page: Upcoming Pokemon TCG Events
// ============================================================

import { useEffect, useState } from 'react';
import { MapPin, Calendar, ExternalLink } from 'lucide-react';
import api from '../services/api.js';

export default function ShowsPage() {
  const [shows, setShows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [zip, setZip] = useState('');

  function loadShows(zipCode = '') {
    setLoading(true);
    api.get(`/shows/upcoming?zip=${zipCode}&radius=200`).then(r => {
      setShows(r.data.shows || []);
      setLoading(false);
    }).catch(() => setLoading(false));
  }

  useEffect(() => { loadShows(); }, []);

  function handleSearch(e) { e.preventDefault(); loadShows(zip); }

  // Compute how many days away an event is
  function daysAway(dateStr) {
    const diff = new Date(dateStr) - new Date();
    const days = Math.ceil(diff / (1000 * 60 * 60 * 24));
    if (days === 0) return 'Today!';
    if (days === 1) return 'Tomorrow!';
    return `In ${days} days`;
  }

  return (
    <div>
      <div className="section-header">
        <div className="section-title"><MapPin size={18} className="icon" /> Upcoming Shows & Events</div>
      </div>

      {/* ZIP code filter */}
      <div className="glass-card" style={{ padding:20, marginBottom:24 }}>
        <form onSubmit={handleSearch} style={{ display:'flex', gap:12 }}>
          <input className="input" value={zip} onChange={e => setZip(e.target.value)} placeholder="Enter ZIP code to find nearby shows..." style={{ flex:1, maxWidth:400 }} />
          <button className="btn btn-primary" type="submit"><MapPin size={15} /> Find Shows</button>
          <button className="btn btn-secondary" type="button" onClick={() => { setZip(''); loadShows(''); }}>Show All</button>
        </form>
      </div>

      {loading
        ? <div style={{ display:'flex', justifyContent:'center', paddingTop:40 }}><div className="spinner" /></div>
        : shows.length > 0
          ? (
            <div className="grid-auto">
              {shows.map((show, i) => (
                <div key={show.id || show.eventbriteId || i} className="glass-card" style={{ padding:24 }}>
                  {/* Days badge */}
                  <div style={{ marginBottom:12 }}>
                    <span className="badge badge-gold">{show.start_date || show.startDate ? daysAway(show.start_date || show.startDate) : 'TBD'}</span>
                  </div>
                  <h3 style={{ fontWeight:700, fontSize:'1rem', marginBottom:12, lineHeight:1.4 }}>{show.name}</h3>
                  {(show.venue_name || show.venueName) && (
                    <div style={{ display:'flex', alignItems:'center', gap:8, marginBottom:8, color:'var(--color-text-secondary)', fontSize:'0.875rem' }}>
                      <MapPin size={14} /> {show.venue_name || show.venueName}
                    </div>
                  )}
                  <div style={{ display:'flex', alignItems:'center', gap:8, color:'var(--color-text-secondary)', fontSize:'0.875rem', marginBottom:8 }}>
                    <Calendar size={14} />
                    {show.start_date || show.startDate
                      ? new Date(show.start_date || show.startDate).toLocaleDateString('en-US', { weekday:'long', month:'long', day:'numeric', year:'numeric' })
                      : 'Date TBD'}
                  </div>
                  {(show.city || show.state) && (
                    <div style={{ fontSize:'0.8125rem', color:'var(--color-text-muted)', marginBottom:16 }}>
                      📍 {[show.city, show.state].filter(Boolean).join(', ')}
                    </div>
                  )}
                  {(show.event_url || show.eventUrl) && (
                    <a href={show.event_url || show.eventUrl} target="_blank" rel="noreferrer" className="btn btn-secondary btn-sm">
                      <ExternalLink size={13} /> View Event
                    </a>
                  )}
                </div>
              ))}
            </div>
          )
          : (
            <div className="glass-card empty-state" style={{ padding:60 }}>
              <MapPin size={48} />
              <h3 style={{ fontWeight:700 }}>No shows found</h3>
              <p>Try expanding your search radius or check back when Eventbrite is connected in Settings</p>
            </div>
          )
      }
    </div>
  );
}
