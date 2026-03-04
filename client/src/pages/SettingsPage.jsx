// ============================================================
// PokémonTool — Settings Page
// Manage API keys (eBay, TCGplayer, Eventbrite) and preferences
// API keys are AES-256 encrypted before being stored server-side
// ============================================================

import { useEffect, useState } from 'react';
import { Settings, Key, Eye, EyeOff, Check, Trash2, AlertCircle } from 'lucide-react';
import api from '../services/api.js';

const PLATFORMS = [
  {
    id:          'ebay',
    name:        'eBay',
    logo:        '🛒',
    description: 'eBay Developer API — required for real-time listings and price alerts.',
    howTo:       'Register at developer.ebay.com → My Apps → Create App → copy App ID',
    required:    true,
  },
  {
    id:          'tcgplayer',
    name:        'TCGPlayer',
    logo:        '🃏',
    description: 'TCGPlayer commercial API for detailed market prices and sale velocity.',
    howTo:       'Apply at tcgplayer.com/developers — commercial agreement required.',
    required:    false,
  },
  {
    id:          'eventbrite',
    name:        'Eventbrite',
    logo:        '🗓️',
    description: 'Eventbrite API for upcoming Pokémon TCG shows and events near you.',
    howTo:       'Get a free token at eventbrite.com/platform → API keys',
    required:    false,
  },
];

export default function SettingsPage() {
  const [savedKeys, setSavedKeys]     = useState([]);  // Platforms that already have keys
  const [keyInputs, setKeyInputs]     = useState({});  // Live input state per platform
  const [showKey, setShowKey]         = useState({});  // Toggle visibility per platform
  const [saving, setSaving]           = useState({});
  const [message, setMessage]         = useState({});  // Success/error messages per platform

  useEffect(() => {
    api.get('/apikeys').then(r => setSavedKeys(r.data.keys.map(k => k.platform)));
  }, []);

  function isSaved(platformId) { return savedKeys.includes(platformId); }

  async function handleSave(platformId) {
    const keyValue = keyInputs[platformId]?.trim();
    if (!keyValue) return setMessage(m => ({...m, [platformId]: { type:'error', text:'Please enter a valid API key.' }}));
    setSaving(s => ({...s, [platformId]: true}));
    try {
      await api.post('/apikeys', { platform: platformId, keyValue });
      setSavedKeys(k => [...k.filter(p => p !== platformId), platformId]);
      setKeyInputs(k => ({...k, [platformId]: ''}));
      setMessage(m => ({...m, [platformId]: { type:'success', text:'API key saved and encrypted successfully! ✓' }}));
    } catch (err) {
      setMessage(m => ({...m, [platformId]: { type:'error', text: err.response?.data?.error || 'Failed to save key.' }}));
    } finally {
      setSaving(s => ({...s, [platformId]: false}));
      setTimeout(() => setMessage(m => ({...m, [platformId]: null})), 4000);
    }
  }

  async function handleDelete(platformId) {
    if (!confirm(`Remove your ${platformId} API key?`)) return;
    await api.delete(`/apikeys/${platformId}`);
    setSavedKeys(k => k.filter(p => p !== platformId));
    setMessage(m => ({...m, [platformId]: { type:'success', text:'Key removed.' }}));
    setTimeout(() => setMessage(m => ({...m, [platformId]: null})), 3000);
  }

  return (
    <div>
      <div className="section-header">
        <div className="section-title"><Settings size={18} className="icon" /> Settings</div>
      </div>

      {/* Security note */}
      <div style={{ padding:'14px 18px', background:'rgba(79,142,247,0.08)', border:'1px solid rgba(79,142,247,0.2)', borderRadius:'var(--radius-md)', marginBottom:24, display:'flex', alignItems:'flex-start', gap:12 }}>
        <Key size={18} color="var(--color-accent-primary)" style={{ flexShrink:0, marginTop:1 }} />
        <div>
          <div style={{ fontWeight:600, fontSize:'0.875rem' }}>Your API keys are encrypted</div>
          <div style={{ fontSize:'0.8125rem', color:'var(--color-text-secondary)', marginTop:3 }}>
            All API keys are encrypted with AES-256-GCM before storage. They are never sent back to your browser and are only used server-side for marketplace data fetching.
          </div>
        </div>
      </div>

      {/* Platform key cards */}
      <div style={{ display:'flex', flexDirection:'column', gap:16 }}>
        {PLATFORMS.map(platform => (
          <div key={platform.id} className="glass-card" style={{ padding:24 }}>
            <div style={{ display:'flex', alignItems:'flex-start', gap:16 }}>
              <div style={{ fontSize:'1.75rem' }}>{platform.logo}</div>
              <div style={{ flex:1 }}>
                <div style={{ display:'flex', alignItems:'center', gap:10, marginBottom:6 }}>
                  <h3 style={{ fontWeight:700, fontSize:'1rem' }}>{platform.name}</h3>
                  {platform.required && <span className="badge badge-alert">Required</span>}
                  {isSaved(platform.id) && <span className="badge badge-rising"><Check size={11} /> Connected</span>}
                </div>
                <p style={{ fontSize:'0.8125rem', color:'var(--color-text-secondary)', marginBottom:10 }}>{platform.description}</p>
                <div style={{ display:'flex', alignItems:'center', gap:6, marginBottom:16, padding:'8px 12px', background:'var(--color-bg-elevated)', borderRadius:'var(--radius-sm)', fontSize:'0.75rem', color:'var(--color-text-muted)' }}>
                  <AlertCircle size={12} /> {platform.howTo}
                </div>
                <div style={{ display:'flex', gap:10 }}>
                  <div style={{ flex:1, position:'relative' }}>
                    <input
                      className="input"
                      type={showKey[platform.id] ? 'text' : 'password'}
                      value={keyInputs[platform.id] || ''}
                      onChange={e => setKeyInputs(k => ({...k, [platform.id]: e.target.value}))}
                      placeholder={isSaved(platform.id) ? '••••••• (key saved — paste new to replace)' : `Paste your ${platform.name} API key here...`}
                      style={{ paddingRight:42 }}
                    />
                    <button onClick={() => setShowKey(s => ({...s, [platform.id]: !s[platform.id]}))} style={{ position:'absolute', right:12, top:'50%', transform:'translateY(-50%)', background:'none', border:'none', color:'var(--color-text-muted)', cursor:'pointer' }}>
                      {showKey[platform.id] ? <EyeOff size={15} /> : <Eye size={15} />}
                    </button>
                  </div>
                  <button className="btn btn-primary" onClick={() => handleSave(platform.id)} disabled={saving[platform.id]}>
                    {saving[platform.id] ? '...' : isSaved(platform.id) ? 'Update Key' : 'Save Key'}
                  </button>
                  {isSaved(platform.id) && (
                    <button className="btn btn-secondary btn-icon" onClick={() => handleDelete(platform.id)} title={`Remove ${platform.name} key`}>
                      <Trash2 size={15} color="var(--color-accent-falling)" />
                    </button>
                  )}
                </div>
                {message[platform.id] && (
                  <div style={{ marginTop:10, fontSize:'0.8125rem', color: message[platform.id].type === 'success' ? 'var(--color-accent-rising)' : 'var(--color-accent-falling)' }}>
                    {message[platform.id].text}
                  </div>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
