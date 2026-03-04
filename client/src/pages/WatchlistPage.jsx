// ============================================================
// PokémonTool — Watchlist Page
// Add/remove cards to monitor, set buy/sell price targets
// ============================================================

import { useEffect, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { Star, Plus, Trash2, Bell } from 'lucide-react';
import api from '../services/api.js';
import { setWatchlist, addWatchlistItem, removeWatchlistItem } from '../store/index.js';

export default function WatchlistPage() {
  const dispatch = useDispatch();
  const items    = useSelector(s => s.watchlist.items);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm]       = useState({ cardName: '', setName: '', targetBuyPrice: '', targetSellPrice: '', notes: '' });
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    api.get('/watchlist').then(r => {
      dispatch(setWatchlist(r.data.watchlist));
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [dispatch]);

  async function handleAdd(e) {
    e.preventDefault();
    setSubmitting(true);
    try {
      const { data } = await api.post('/watchlist', form);
      dispatch(addWatchlistItem(data.item));
      setShowAdd(false);
      setForm({ cardName: '', setName: '', targetBuyPrice: '', targetSellPrice: '', notes: '' });
    } catch (err) {
      alert(err.response?.data?.error || 'Could not add card');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleRemove(id, name) {
    if (!confirm(`Remove "${name}" from watchlist?`)) return;
    await api.delete(`/watchlist/${id}`);
    dispatch(removeWatchlistItem(id));
  }

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 60 }}><div className="spinner" /></div>;

  return (
    <div>
      <div className="section-header">
        <div className="section-title"><Star size={18} className="icon" /> My Watchlist ({items.length})</div>
        <button className="btn btn-primary" onClick={() => setShowAdd(s => !s)}><Plus size={16} /> Add Card</button>
      </div>

      {/* Add card form */}
      {showAdd && (
        <div className="glass-card" style={{ padding: 24, marginBottom: 24 }}>
          <h3 style={{ fontWeight: 700, marginBottom: 20 }}>Add Card to Watchlist</h3>
          <form onSubmit={handleAdd}>
            <div className="grid-2">
              <div className="form-group"><label className="label">Card Name *</label><input className="input" value={form.cardName} onChange={e => setForm(f => ({...f, cardName: e.target.value}))} placeholder="e.g. Charizard Base Set" required /></div>
              <div className="form-group"><label className="label">Set Name</label><input className="input" value={form.setName} onChange={e => setForm(f => ({...f, setName: e.target.value}))} placeholder="e.g. Base Set" /></div>
              <div className="form-group"><label className="label">Buy Alert Price ($)</label><input className="input" type="number" step="0.01" value={form.targetBuyPrice} onChange={e => setForm(f => ({...f, targetBuyPrice: e.target.value}))} placeholder="Alert when price drops below..." /></div>
              <div className="form-group"><label className="label">Sell Alert Price ($)</label><input className="input" type="number" step="0.01" value={form.targetSellPrice} onChange={e => setForm(f => ({...f, targetSellPrice: e.target.value}))} placeholder="Alert when price rises above..." /></div>
            </div>
            <div className="form-group"><label className="label">Notes</label><input className="input" value={form.notes} onChange={e => setForm(f => ({...f, notes: e.target.value}))} placeholder="Optional notes about this card" /></div>
            <div style={{ display: 'flex', gap: 12 }}>
              <button className="btn btn-primary" type="submit" disabled={submitting}>{submitting ? 'Adding...' : 'Add to Watchlist'}</button>
              <button className="btn btn-secondary" type="button" onClick={() => setShowAdd(false)}>Cancel</button>
            </div>
          </form>
        </div>
      )}

      {/* Watchlist table */}
      {items.length > 0 ? (
        <div className="glass-card" style={{ overflow: 'hidden' }}>
          <table className="data-table">
            <thead>
              <tr>
                <th>Card Name</th><th>Set</th><th>Buy Alert</th><th>Sell Alert</th><th>Notes</th><th>Added</th><th></th>
              </tr>
            </thead>
            <tbody>
              {items.map(item => (
                <tr key={item.id}>
                  <td><strong>{item.card_name}</strong></td>
                  <td style={{ color: 'var(--color-text-secondary)' }}>{item.set_name || '—'}</td>
                  <td>{item.target_buy_price ? <span className="badge badge-rising">≤ ${item.target_buy_price}</span> : <span style={{ color: 'var(--color-text-muted)' }}>—</span>}</td>
                  <td>{item.target_sell_price ? <span className="badge badge-falling">≥ ${item.target_sell_price}</span> : <span style={{ color: 'var(--color-text-muted)' }}>—</span>}</td>
                  <td style={{ color: 'var(--color-text-muted)', fontSize: '0.8125rem' }}>{item.notes || '—'}</td>
                  <td style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>{new Date(item.added_at).toLocaleDateString()}</td>
                  <td>
                    <button className="btn btn-icon btn-secondary" onClick={() => handleRemove(item.id, item.card_name)} title="Remove from watchlist"><Trash2 size={15} color="var(--color-accent-falling)" /></button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="glass-card empty-state" style={{ padding: 60 }}>
          <Bell size={48} />
          <h3 style={{ fontWeight: 700 }}>Your watchlist is empty</h3>
          <p>Add cards above to start receiving price alerts automatically</p>
          <button className="btn btn-primary" onClick={() => setShowAdd(true)}><Plus size={16} /> Add Your First Card</button>
        </div>
      )}
    </div>
  );
}
