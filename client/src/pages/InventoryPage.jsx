// ============================================================
// PokémonTool — Inventory Page
// Track owned cards with market value and position P&L.
// ============================================================

import { useEffect, useMemo, useRef, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { Download, Package, Plus, Search, ShoppingBag, Trash2, Upload, X } from 'lucide-react';
import api from '../services/api.js';
import { setInventory, removeInventoryItem } from '../store/index.js';

const initialForm = { query: '', condition: 'NM', quantity: 1, purchasePrice: '', notes: '' };

function read(item, camelKey, snakeKey = camelKey) {
  return item?.[camelKey] ?? item?.[snakeKey];
}

function money(value) {
  const num = Number(value);
  return Number.isFinite(num) ? `$${num.toFixed(2)}` : '-';
}

function effectiveMarketPrice(card) {
  const primary = Number(card?.market);
  if (Number.isFinite(primary)) return primary;
  const trend = Number(card?.cardmarketTrend);
  return Number.isFinite(trend) ? trend : null;
}

function toNumberOrNull(value) {
  if (value === '' || value === null || value === undefined) return null;
  const num = Number(value);
  return Number.isFinite(num) ? num : null;
}

export default function InventoryPage() {
  const dispatch = useDispatch();
  const { items, total } = useSelector(s => s.inventory);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm] = useState(initialForm);
  const [selectedCard, setSelectedCard] = useState(null);
  const [results, setResults] = useState([]);
  const [searching, setSearching] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [storeBusyId, setStoreBusyId] = useState('');
  const fileRef = useRef();

  const totals = useMemo(() => {
    return items.reduce((acc, item) => {
      const qty = Number(read(item, 'quantity')) || 1;
      const paid = Number(read(item, 'purchasePrice', 'purchase_price'));
      const current = Number(read(item, 'currentValue', 'current_value'));
      if (Number.isFinite(paid)) acc.cost += paid * qty;
      if (Number.isFinite(current)) acc.value += current * qty;
      return acc;
    }, { cost: 0, value: 0 });
  }, [items]);
  const totalPL = totals.value - totals.cost;

  useEffect(() => {
    api.get('/inventory').then(r => {
      dispatch(setInventory({ inventory: r.data.inventory || [], total: r.data.total || 0 }));
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [dispatch]);

  function resetAddForm() {
    setForm(initialForm);
    setSelectedCard(null);
    setResults([]);
  }

  async function refreshInventory() {
    const r = await api.get('/inventory');
    dispatch(setInventory({ inventory: r.data.inventory || [], total: r.data.total || 0 }));
  }

  async function handleSearch(e) {
    e.preventDefault();
    if (!form.query.trim()) return;
    setSearching(true);
    try {
      const { data } = await api.get(`/cards/tcg-search?q=${encodeURIComponent(form.query)}&limit=8`);
      setResults(data.cards || []);
    } catch (err) {
      alert(err.response?.data?.error || 'Card search failed');
    } finally {
      setSearching(false);
    }
  }

  function selectCard(card) {
    setSelectedCard(card);
  }

  async function handleAdd(e) {
    e.preventDefault();
    if (!selectedCard) return;
    setSubmitting(true);
    try {
      await api.post('/inventory', {
        cardName: selectedCard.name,
        setName: selectedCard.set,
        cardNumber: selectedCard.number,
        externalCardId: selectedCard.id,
        rarity: selectedCard.rarity,
        imageUrl: selectedCard.image,
        marketUpdatedAt: selectedCard.tcgplayerUpdatedAt || selectedCard.cardmarketUpdatedAt || '',
        priceSource: selectedCard.market ? 'poketcg_tcgplayer' : (selectedCard.cardmarketTrend ? 'poketcg_cardmarket' : 'poketcg'),
        condition: form.condition,
        quantity: Number(form.quantity) || 1,
        purchasePrice: toNumberOrNull(form.purchasePrice),
        currentValue: effectiveMarketPrice(selectedCard),
        notes: form.notes,
      });
      await refreshInventory();
      setShowAdd(false);
      resetAddForm();
    } catch (err) {
      alert(err.response?.data?.error || 'Could not add card');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(id) {
    await api.delete(`/inventory/${id}`);
    dispatch(removeInventoryItem(id));
  }

  async function handleAddToStore(item) {
    const id = read(item, 'id');
    const suggested = read(item, 'targetSalePrice', 'target_sale_price') ?? read(item, 'currentValue', 'current_value') ?? read(item, 'purchasePrice', 'purchase_price');
    const rawPrice = window.prompt('Store price for this card', suggested ? Number(suggested).toFixed(2) : '');
    if (rawPrice === null) return;
    const storePrice = toNumberOrNull(rawPrice);
    if (storePrice === null || storePrice <= 0) {
      alert('Enter a valid store price');
      return;
    }
    setStoreBusyId(id);
    try {
      await api.post(`/inventory/${id}/store-listing`, { storePrice });
      await refreshInventory();
    } catch (err) {
      alert(err.response?.data?.error || 'Could not mark card ready for store');
    } finally {
      setStoreBusyId('');
    }
  }

  async function handleImport(e) {
    const file = e.target.files[0];
    if (!file) return;
    const formData = new FormData();
    formData.append('file', file);
    const { data } = await api.post('/inventory/import', formData, { headers: { 'Content-Type': 'multipart/form-data' } });
    alert(data.message);
    await refreshInventory();
  }

  async function handleExport() {
    const r = await api.get('/inventory/export', { responseType: 'blob' });
    const url = URL.createObjectURL(new Blob([r.data]));
    const a = document.createElement('a'); a.href = url; a.download = 'pokemontool_inventory.csv'; a.click();
  }

  if (loading) return <div style={{ display:'flex', justifyContent:'center', paddingTop:60 }}><div className="spinner" /></div>;

  return (
    <div>
      <div className="section-header">
        <div className="section-title"><Package size={18} className="icon" /> Inventory ({total} cards)</div>
        <div style={{ display:'flex', gap:10, flexWrap:'wrap' }}>
          <input ref={fileRef} type="file" accept=".csv" onChange={handleImport} style={{ display:'none' }} />
          <button className="btn btn-secondary" type="button" onClick={() => fileRef.current.click()}><Upload size={15} /> Import CSV</button>
          <button className="btn btn-secondary" type="button" onClick={handleExport}><Download size={15} /> Export CSV</button>
          <button className="btn btn-primary" type="button" onClick={() => setShowAdd(s => !s)}><Plus size={15} /> Add Card</button>
        </div>
      </div>

      <div className="grid-3" style={{ marginBottom:24 }}>
        <div className="stat-tile glass-card" style={{ padding:20 }}><div className="stat-label">Cost Basis</div><div className="stat-value">{money(totals.cost)}</div></div>
        <div className="stat-tile glass-card" style={{ padding:20 }}><div className="stat-label">Market Value</div><div className="stat-value">{money(totals.value)}</div></div>
        <div className="stat-tile glass-card" style={{ padding:20 }}><div className="stat-label">Unrealized P&L</div><div className="stat-value" style={{ color: totalPL >= 0 ? 'var(--color-accent-rising)' : 'var(--color-accent-falling)' }}>{money(totalPL)}</div></div>
      </div>

      {showAdd && (
        <div className="glass-card" style={{ padding:24, marginBottom:24 }}>
          <div style={{ display:'flex', justifyContent:'space-between', alignItems:'center', gap:16, marginBottom:20 }}>
            <h3 style={{ fontWeight:700 }}>Add Owned Card</h3>
            <button className="btn btn-icon btn-secondary" type="button" onClick={() => { setShowAdd(false); resetAddForm(); }} title="Close"><X size={16} /></button>
          </div>

          <form onSubmit={handleSearch} style={{ marginBottom:18 }}>
            <div style={{ display:'flex', gap:12, alignItems:'flex-end' }}>
              <div className="form-group" style={{ flex:1, marginBottom:0 }}>
                <label className="label">Find Card</label>
                <input className="input" value={form.query} onChange={e => setForm(f => ({ ...f, query:e.target.value }))} placeholder="Radiant Charizard, Pikachu, Umbreon..." />
              </div>
              <button className="btn btn-secondary" type="submit" disabled={searching || !form.query.trim()}><Search size={16} /> {searching ? 'Searching...' : 'Search'}</button>
            </div>
          </form>

          {results.length > 0 && (
            <div style={{ display:'grid', gridTemplateColumns:'repeat(auto-fit, minmax(240px, 1fr))', gap:12, marginBottom:20 }}>
              {results.map(card => {
                const active = selectedCard?.id === card.id;
                return (
                  <button key={card.id} type="button" onClick={() => selectCard(card)} className="glass-card" style={{ padding:12, textAlign:'left', border: active ? '1px solid var(--color-accent-primary)' : '1px solid var(--color-border)', cursor:'pointer', display:'flex', gap:12, alignItems:'center' }}>
                    {card.image && <img src={card.image} alt={card.name} style={{ width:44, height:62, objectFit:'cover', borderRadius:6 }} />}
                    <span style={{ minWidth:0 }}>
                      <span style={{ display:'block', fontWeight:700, fontSize:'0.875rem' }}>{card.name}</span>
                      <span style={{ display:'block', color:'var(--color-text-muted)', fontSize:'0.75rem' }}>{card.set} #{card.number}</span>
                      <span className="price-tag" style={{ display:'block', fontSize:'0.9rem', marginTop:4 }}>{money(effectiveMarketPrice(card))}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          )}

          <form onSubmit={handleAdd}>
            <div style={{ padding:16, marginBottom:18, border:'1px solid var(--color-border)', borderRadius:8, display:'flex', gap:14, alignItems:'center' }}>
              {selectedCard?.image && <img src={selectedCard.image} alt={selectedCard.name} style={{ width:52, height:74, objectFit:'cover', borderRadius:6 }} />}
              <div style={{ flex:1, minWidth:0 }}>
                <div style={{ color:'var(--color-text-muted)', fontSize:'0.75rem', marginBottom:4 }}>Selected Card</div>
                <div style={{ fontWeight:700 }}>{selectedCard ? selectedCard.name : 'None selected'}</div>
                <div style={{ color:'var(--color-text-secondary)', fontSize:'0.8125rem' }}>{selectedCard ? `${selectedCard.set} #${selectedCard.number}` : 'Search and click a result above'}</div>
              </div>
              <div style={{ textAlign:'right' }}>
                <div style={{ color:'var(--color-text-muted)', fontSize:'0.75rem', marginBottom:4 }}>Market</div>
                <div className="price-tag" style={{ fontSize:'1rem' }}>{selectedCard ? money(effectiveMarketPrice(selectedCard)) : '-'}</div>
              </div>
            </div>

            <div className="grid-3">
              <div className="form-group"><label className="label">Condition</label><select className="input" value={form.condition} onChange={e => setForm(f => ({ ...f, condition:e.target.value }))}><option>NM</option><option>LP</option><option>MP</option><option>HP</option><option>Damaged</option></select></div>
              <div className="form-group"><label className="label">Qty</label><input className="input" type="number" min="1" value={form.quantity} onChange={e => setForm(f => ({ ...f, quantity:e.target.value }))} /></div>
              <div className="form-group"><label className="label">Paid Per Card ($)</label><input className="input" type="number" step="0.01" value={form.purchasePrice} onChange={e => setForm(f => ({ ...f, purchasePrice:e.target.value }))} placeholder="0.00" /></div>
            </div>
            <div className="form-group"><label className="label">Notes</label><input className="input" value={form.notes} onChange={e => setForm(f => ({ ...f, notes:e.target.value }))} placeholder="Where you bought it, grading plan, fees, etc." /></div>
            <div style={{ display:'flex', gap:12, flexWrap:'wrap' }}>
              <button className="btn btn-primary" type="submit" disabled={submitting || !selectedCard}>{submitting ? 'Adding...' : 'Add to Inventory'}</button>
              <button className="btn btn-secondary" type="button" onClick={resetAddForm}>Clear</button>
            </div>
          </form>
        </div>
      )}

      {items.length > 0 ? (
        <div className="glass-card" style={{ overflow:'hidden' }}>
          <table className="data-table">
            <thead><tr><th>Card</th><th>Condition</th><th>Qty</th><th>Paid</th><th>Market</th><th>Position</th><th>P&L</th><th>Store</th><th></th></tr></thead>
            <tbody>
              {items.map(item => {
                const cardName = read(item, 'cardName', 'card_name');
                const setName = read(item, 'setName', 'set_name');
                const imageUrl = read(item, 'imageUrl', 'image_url');
                const qty = Number(read(item, 'quantity')) || 1;
                const paid = Number(read(item, 'purchasePrice', 'purchase_price'));
                const current = Number(read(item, 'currentValue', 'current_value'));
                const position = Number.isFinite(current) ? current * qty : null;
                const pl = Number.isFinite(current) && Number.isFinite(paid) ? (current - paid) * qty : null;
                const storeStatus = read(item, 'storeListingStatus', 'store_listing_status') || 'NOT_LISTED';
                const storePrice = Number(read(item, 'storePrice', 'store_price'));
                return (
                  <tr key={item.id}>
                    <td>
                      <div style={{ display:'flex', alignItems:'center', gap:10 }}>
                        {imageUrl && <img src={imageUrl} alt={cardName} style={{ width:34, height:48, objectFit:'cover', borderRadius:5 }} />}
                        <div><strong>{cardName}</strong><div style={{ color:'var(--color-text-secondary)', fontSize:'0.75rem' }}>{setName || '-'}</div></div>
                      </div>
                    </td>
                    <td><span className="badge badge-stable">{read(item, 'condition') || '-'}</span></td>
                    <td style={{ textAlign:'center' }}>{qty}</td>
                    <td>{Number.isFinite(paid) ? money(paid) : '-'}</td>
                    <td>{Number.isFinite(current) ? <strong style={{ color:'var(--color-accent-gold)' }}>{money(current)}</strong> : '-'}</td>
                    <td>{position != null ? money(position) : '-'}</td>
                    <td>{pl != null ? <span style={{ color: pl >= 0 ? 'var(--color-accent-rising)' : 'var(--color-accent-falling)', fontWeight:600 }}>{pl >= 0 ? '+' : ''}{money(pl)}</span> : '-'}</td>
                    <td>
                      <div style={{ display:'flex', flexDirection:'column', gap:6, alignItems:'flex-start' }}>
                        <span className="badge" style={{ color: storeStatus === 'SYNCED' ? 'var(--color-accent-rising)' : storeStatus === 'READY' ? 'var(--color-accent-gold)' : 'var(--color-text-muted)' }}>{storeStatus === 'NOT_LISTED' ? 'Not listed' : storeStatus}</span>
                        {Number.isFinite(storePrice) && <span style={{ fontSize:'0.75rem', color:'var(--color-text-secondary)' }}>{money(storePrice)}</span>}
                      </div>
                    </td>
                    <td>
                      <div style={{ display:'flex', gap:6, justifyContent:'flex-end' }}>
                        <button className="btn btn-icon btn-secondary" type="button" onClick={() => handleAddToStore(item)} disabled={storeBusyId === item.id} title="Add to store"><ShoppingBag size={14} /></button>
                        <button className="btn btn-icon btn-secondary" type="button" onClick={() => handleDelete(item.id)} title="Remove from inventory"><Trash2 size={14} color="var(--color-accent-falling)" /></button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="glass-card empty-state" style={{ padding:60 }}>
          <Package size={48} />
          <h3 style={{ fontWeight:700 }}>Inventory is empty</h3>
          <p>Add market-priced cards or import your existing collection as a CSV file</p>
          <div style={{ display:'flex', gap:12 }}>
            <button className="btn btn-primary" type="button" onClick={() => setShowAdd(true)}><Plus size={15} /> Add Card</button>
            <button className="btn btn-secondary" type="button" onClick={() => fileRef.current.click()}><Upload size={15} /> Import CSV</button>
          </div>
        </div>
      )}
    </div>
  );
}
