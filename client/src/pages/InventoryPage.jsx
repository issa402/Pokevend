// ============================================================
// PokémonTool — Inventory Page
// Track personal card collection with CSV import/export
// ============================================================

import { useEffect, useState, useRef } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { Package, Upload, Download, Plus, Trash2 } from 'lucide-react';
import api from '../services/api.js';
import { setInventory, removeInventoryItem } from '../store/index.js';

export default function InventoryPage() {
  const dispatch   = useDispatch();
  const { items, total } = useSelector(s => s.inventory);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm]       = useState({ cardName:'', setName:'', cardNumber:'', condition:'NM', quantity:1, purchasePrice:'', notes:'' });
  const fileRef = useRef();

  useEffect(() => {
    api.get('/inventory').then(r => {
      dispatch(setInventory({ inventory: r.data.inventory, total: r.data.total }));
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [dispatch]);

  async function handleAdd(e) {
    e.preventDefault();
    await api.post('/inventory', form);
    const r = await api.get('/inventory');
    dispatch(setInventory({ inventory: r.data.inventory, total: r.data.total }));
    setShowAdd(false);
    setForm({ cardName:'', setName:'', cardNumber:'', condition:'NM', quantity:1, purchasePrice:'', notes:'' });
  }

  async function handleDelete(id) {
    await api.delete(`/inventory/${id}`);
    dispatch(removeInventoryItem(id));
  }

  async function handleImport(e) {
    const file = e.target.files[0];
    if (!file) return;
    const formData = new FormData();
    formData.append('file', file);
    const { data } = await api.post('/inventory/import', formData, { headers: { 'Content-Type': 'multipart/form-data' } });
    alert(data.message);
    const r = await api.get('/inventory');
    dispatch(setInventory({ inventory: r.data.inventory, total: r.data.total }));
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
        <div style={{ display:'flex', gap:10 }}>
          <input ref={fileRef} type="file" accept=".csv" onChange={handleImport} style={{ display:'none' }} />
          <button className="btn btn-secondary" onClick={() => fileRef.current.click()}><Upload size={15} /> Import CSV</button>
          <button className="btn btn-secondary" onClick={handleExport}><Download size={15} /> Export CSV</button>
          <button className="btn btn-primary" onClick={() => setShowAdd(s => !s)}><Plus size={15} /> Add Card</button>
        </div>
      </div>

      {showAdd && (
        <div className="glass-card" style={{ padding:24, marginBottom:24 }}>
          <h3 style={{ fontWeight:700, marginBottom:20 }}>Add Card to Inventory</h3>
          <form onSubmit={handleAdd}>
            <div className="grid-3">
              <div className="form-group"><label className="label">Card Name *</label><input className="input" value={form.cardName} onChange={e => setForm(f=>({...f,cardName:e.target.value}))} required placeholder="Charizard Base Set" /></div>
              <div className="form-group"><label className="label">Set</label><input className="input" value={form.setName} onChange={e => setForm(f=>({...f,setName:e.target.value}))} placeholder="Base Set" /></div>
              <div className="form-group"><label className="label">Card #</label><input className="input" value={form.cardNumber} onChange={e => setForm(f=>({...f,cardNumber:e.target.value}))} placeholder="4/102" /></div>
              <div className="form-group"><label className="label">Condition</label><select className="input" value={form.condition} onChange={e => setForm(f=>({...f,condition:e.target.value}))}><option>NM</option><option>LP</option><option>MP</option><option>HP</option><option>Damaged</option></select></div>
              <div className="form-group"><label className="label">Qty</label><input className="input" type="number" min="1" value={form.quantity} onChange={e => setForm(f=>({...f,quantity:e.target.value}))} /></div>
              <div className="form-group"><label className="label">Purchase Price ($)</label><input className="input" type="number" step="0.01" value={form.purchasePrice} onChange={e => setForm(f=>({...f,purchasePrice:e.target.value}))} placeholder="0.00" /></div>
            </div>
            <div style={{ display:'flex', gap:12 }}>
              <button className="btn btn-primary" type="submit">Add Card</button>
              <button className="btn btn-secondary" type="button" onClick={() => setShowAdd(false)}>Cancel</button>
            </div>
          </form>
        </div>
      )}

      {items.length > 0 ? (
        <div className="glass-card" style={{ overflow:'hidden' }}>
          <table className="data-table">
            <thead><tr><th>Card</th><th>Set</th><th>#</th><th>Condition</th><th>Qty</th><th>Paid</th><th>Current Value</th><th>P&L</th><th></th></tr></thead>
            <tbody>
              {items.map(item => {
                const pl = item.current_value && item.purchase_price ? ((item.current_value - item.purchase_price) * item.quantity) : null;
                return (
                  <tr key={item.id}>
                    <td><strong>{item.card_name}</strong></td>
                    <td style={{ color:'var(--color-text-secondary)' }}>{item.set_name||'—'}</td>
                    <td style={{ color:'var(--color-text-muted)' }}>{item.card_number||'—'}</td>
                    <td><span className="badge badge-stable">{item.condition}</span></td>
                    <td style={{ textAlign:'center' }}>{item.quantity}</td>
                    <td>{item.purchase_price ? `$${parseFloat(item.purchase_price).toFixed(2)}` : '—'}</td>
                    <td>{item.current_value ? <strong style={{ color:'var(--color-accent-gold)' }}>${parseFloat(item.current_value).toFixed(2)}</strong> : '—'}</td>
                    <td>{pl != null ? <span style={{ color: pl >= 0 ? 'var(--color-accent-rising)' : 'var(--color-accent-falling)', fontWeight:600 }}>{pl >= 0 ? '+' : ''}${pl.toFixed(2)}</span> : '—'}</td>
                    <td><button className="btn btn-icon btn-secondary" onClick={() => handleDelete(item.id)}><Trash2 size={14} color="var(--color-accent-falling)" /></button></td>
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
          <p>Add cards manually or import your existing collection as a CSV file</p>
          <div style={{ display:'flex', gap:12 }}>
            <button className="btn btn-primary" onClick={() => setShowAdd(true)}><Plus size={15} /> Add Card</button>
            <button className="btn btn-secondary" onClick={() => fileRef.current.click()}><Upload size={15} /> Import CSV</button>
          </div>
        </div>
      )}
    </div>
  );
}
