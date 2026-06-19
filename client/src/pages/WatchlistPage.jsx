// ============================================================
// PokémonTool — Watchlist Page
// Market-first watchlist creation powered by PokeTCG.
// ============================================================

import { useEffect, useMemo, useState } from 'react';
import { useDispatch, useSelector } from 'react-redux';
import { Bell, DollarSign, Percent, Plus, Search, Star, Trash2, X } from 'lucide-react';
import api from '../services/api.js';
import { setWatchlist, addWatchlistItem, removeWatchlistItem } from '../store/index.js';

const initialForm = {
  query: '',
  assetType: 'RAW',
  slabTier: 'PSA_10',
  targetBuyPrice: '',
  targetSellPrice: '',
  targetDiscountPct: '15',
  languagePreference: 'BOTH',
  notes: '',
};

const slabOptions = [
  { value: 'PSA_10', label: 'PSA 10' },
  { value: 'PSA_9', label: 'PSA 9' },
  { value: 'PSA_8', label: 'PSA 8' },
  { value: 'PSA_7', label: 'PSA 7' },
  { value: 'PSA_6', label: 'PSA 6' },
  { value: 'PSA_5', label: 'PSA 5' },
  { value: 'PSA_4', label: 'PSA 4' },
  { value: 'PSA_3', label: 'PSA 3' },
  { value: 'PSA_2', label: 'PSA 2' },
  { value: 'PSA_1', label: 'PSA 1' },
  { value: 'CGC_10', label: 'CGC 10' },
  { value: 'CGC_9_5', label: 'CGC 9.5' },
  { value: 'CGC_9', label: 'CGC 9' },
  { value: 'CGC_8_5', label: 'CGC 8.5' },
  { value: 'CGC_8', label: 'CGC 8' },
  { value: 'CGC_7_5', label: 'CGC 7.5' },
  { value: 'CGC_7', label: 'CGC 7' },
  { value: 'CGC_6', label: 'CGC 6' },
  { value: 'CGC_5', label: 'CGC 5' },
  { value: 'CGC_4', label: 'CGC 4' },
  { value: 'CGC_3', label: 'CGC 3' },
  { value: 'CGC_2', label: 'CGC 2' },
  { value: 'CGC_1', label: 'CGC 1' },
  { value: 'BGS_10', label: 'BGS 10' },
  { value: 'BGS_9_5', label: 'BGS 9.5' },
  { value: 'BGS_9', label: 'BGS 9' },
  { value: 'BGS_8_5', label: 'BGS 8.5' },
  { value: 'BGS_8', label: 'BGS 8' },
  { value: 'BGS_7_5', label: 'BGS 7.5' },
  { value: 'BGS_7', label: 'BGS 7' },
  { value: 'BGS_6', label: 'BGS 6' },
  { value: 'BGS_5', label: 'BGS 5' },
  { value: 'BGS_4', label: 'BGS 4' },
  { value: 'BGS_3', label: 'BGS 3' },
  { value: 'BGS_2', label: 'BGS 2' },
  { value: 'BGS_1', label: 'BGS 1' },
  { value: 'BGS_BLACK_LABEL', label: 'BGS Black Label' },
];

const summaryLanes = [
  { value: 'RAW', label: 'Raw' },
  ...slabOptions.filter(option => option.value !== 'BGS_BLACK_LABEL'),
];

const languageOptions = [
  { value: 'BOTH', label: 'English + Japanese' },
  { value: 'ENGLISH', label: 'English' },
  { value: 'JAPANESE', label: 'Japanese' },
];

function slabParts(slabTier) {
  if (!slabTier || slabTier === 'RAW') return { grader: '', grade: '' };
  if (slabTier === 'BGS_BLACK_LABEL') return { grader: 'BGS', grade: '10' };
  const [grader, ...gradeParts] = slabTier.split('_');
  return { grader, grade: gradeParts.join('.').replace('_', '.') };
}

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

function marketSourceLabel(card) {
  if (Number.isFinite(Number(card?.market))) return 'TCGplayer';
  if (Number.isFinite(Number(card?.cardmarketTrend))) return 'Cardmarket trend';
  return 'No market price';
}

function pct(value) {
  const num = Number(value);
  return Number.isFinite(num) ? `${num.toFixed(0)}%` : '-';
}

function toNumberOrNull(value) {
  if (value === '' || value === null || value === undefined) return null;
  const num = Number(value);
  return Number.isFinite(num) ? num : null;
}


async function refreshSlabObservations(item) {
  const cardName = read(item, 'cardName', 'card_name');
  const externalCardId = read(item, 'externalCardId', 'external_card_id');
  const setName = read(item, 'setName', 'set_name') || '';
  const cardNumber = read(item, 'cardNumber', 'card_number') || '';
  const languagePreference = read(item, 'languagePreference', 'language_preference') || 'BOTH';
  if (!cardName || !externalCardId) return;
  const tierRequests = slabOptions.filter(option => option.value !== 'BGS_BLACK_LABEL').map(option => ({ assetType: 'SLAB', slabTier: option.value, pages: '3' }));
  const requests = [
    { assetType: 'RAW', slabTier: '', pages: '5' },
    { assetType: 'SLAB', slabTier: '', pages: '5' },
    ...tierRequests,
  ];
  await Promise.allSettled(requests.map(request => {
    const params = new URLSearchParams({
      cardName,
      externalCardId,
      setName,
      cardNumber,
      assetType: request.assetType,
      languagePreference,
      pages: request.pages,
      publish: 'true',
    });
    if (request.slabTier) params.set('slabTier', request.slabTier);
    return api.get(`/cards/ebay-listings?${params.toString()}`);
  }));
}

export default function WatchlistPage() {
  const dispatch = useDispatch();
  const items = useSelector(s => s.watchlist.items);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm] = useState(initialForm);
  const [selectedCard, setSelectedCard] = useState(null);
  const [results, setResults] = useState([]);
  const [slabSummary, setSlabSummary] = useState([]);
  const [loadingSlabs, setLoadingSlabs] = useState(false);
  const [expandedSlabCard, setExpandedSlabCard] = useState(null);
  const [watchlistSlabs, setWatchlistSlabs] = useState({});
  const [loadingWatchlistSlabs, setLoadingWatchlistSlabs] = useState(null);
  const [importTextByItem, setImportTextByItem] = useState({});
  const [importingItemId, setImportingItemId] = useState(null);
  const [liveListings, setLiveListings] = useState([]);
  const [loadingLiveListings, setLoadingLiveListings] = useState(false);
  const [searching, setSearching] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const calculatedTarget = useMemo(() => {
    const market = effectiveMarketPrice(selectedCard);
    const discount = Number(form.targetDiscountPct);
    if (!Number.isFinite(market) || !Number.isFinite(discount)) return '';
    return (market * (1 - discount / 100)).toFixed(2);
  }, [selectedCard, form.targetDiscountPct]);

  const visibleSummaryRows = useMemo(() => {
    const byTier = new Map(slabSummary.map(row => [row.slabTier || row.slab_tier, row]));
    return summaryRowsFromMap(byTier);
  }, [slabSummary]);

  useEffect(() => {
    api.get('/watchlist').then(r => {
      dispatch(setWatchlist(r.data.watchlist || []));
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [dispatch]);

  function resetAddForm() {
    setForm(initialForm);
    setSelectedCard(null);
    setResults([]);
    setSlabSummary([]);
    setLiveListings([]);
  }

  async function handleMarketSearch(e) {
    e.preventDefault();
    if (!form.query.trim()) return;

    setSearching(true);
    try {
      const { data } = await api.get(`/cards/tcg-search?q=${encodeURIComponent(form.query)}&limit=8`);
      setResults(data.cards || []);
    } catch (err) {
      alert(err.response?.data?.error || 'Market search failed');
    } finally {
      setSearching(false);
    }
  }

  async function selectMarketCard(card) {
    setSelectedCard(card);
    setLiveListings([]);
    setForm(f => ({
      ...f,
      targetBuyPrice: effectiveMarketPrice(card) ? (effectiveMarketPrice(card) * (1 - Number(f.targetDiscountPct || 0) / 100)).toFixed(2) : f.targetBuyPrice,
    }));
    setLoadingSlabs(true);
    try {
      const { data } = await api.get(`/cards/${encodeURIComponent(card.id)}/slab-summary?languagePreference=${encodeURIComponent(form.languagePreference)}`);
      setSlabSummary(data.summary || []);
    } catch {
      setSlabSummary([]);
    } finally {
      setLoadingSlabs(false);
    }
  }

  async function searchLiveListings() {
    if (!selectedCard || form.assetType === 'ALL_SLABS') return;
    setLoadingLiveListings(true);
    try {
      const params = new URLSearchParams({
        cardName: selectedCard.name,
        externalCardId: selectedCard.id,
        setName: selectedCard.set || '',
        cardNumber: selectedCard.number || '',
        assetType: form.assetType,
        languagePreference: form.languagePreference,
        pages: '2',
      });
      if (form.assetType === 'SLAB') params.set('slabTier', form.slabTier);
      const { data } = await api.get(`/cards/ebay-listings?${params.toString()}`);
      setLiveListings(data.listings || []);
    } catch (err) {
      alert(err.response?.data?.error || 'Live eBay lookup failed');
      setLiveListings([]);
    } finally {
      setLoadingLiveListings(false);
    }
  }

  function summaryRowsFromMap(byTier) {
    const fixedRows = summaryLanes.map(lane => {
      const observed = byTier.get(lane.value);
      return observed ? { ...observed, slabTier: lane.value, label: observed.label || lane.label } : {
        slabTier: lane.value,
        label: lane.label,
        lowestPrice: null,
        count: 0,
        listingUrl: '',
      };
    });
    const fixedTiers = new Set(summaryLanes.map(lane => lane.value));
    const dynamicRows = Array.from(byTier.entries())
      .filter(([tier, row]) => !fixedTiers.has(tier) && row && (row.count || row.lowestPrice))
      .map(([tier, row]) => ({ ...row, slabTier: tier, label: row.label || tier }));
    return [...fixedRows, ...dynamicRows];
  }

  async function importEbayText(item) {
    const itemId = read(item, 'id');
    const text = importTextByItem[itemId] || '';
    const cardName = read(item, 'cardName', 'card_name');
    const externalCardId = read(item, 'externalCardId', 'external_card_id');
    if (!text.trim() || !cardName || !externalCardId) return;

    setImportingItemId(itemId);
    try {
      const languagePreference = read(item, 'languagePreference', 'language_preference') || 'BOTH';
      await api.post('/cards/ebay-import-text', {
        cardName,
        externalCardId,
        setName: read(item, 'setName', 'set_name') || '',
        cardNumber: read(item, 'cardNumber', 'card_number') || '',
        assetType: 'SLAB',
        languagePreference,
        publish: true,
        text,
      });
      await new Promise(resolve => setTimeout(resolve, 700));
      const { data } = await api.get(`/cards/${encodeURIComponent(externalCardId)}/slab-summary?languagePreference=${encodeURIComponent(languagePreference)}`);
      setWatchlistSlabs(current => ({ ...current, [itemId]: data.summary || [] }));
      setImportTextByItem(current => ({ ...current, [itemId]: '' }));
    } catch (err) {
      alert(err.response?.data?.error || 'Could not import eBay text');
    } finally {
      setImportingItemId(null);
    }
  }

  async function toggleWatchlistSlabs(item) {
    const itemId = read(item, 'id');
    const externalCardId = read(item, 'externalCardId', 'external_card_id');
    if (!externalCardId) return;

    if (expandedSlabCard === itemId) {
      setExpandedSlabCard(null);
      return;
    }

    setExpandedSlabCard(itemId);

    setLoadingWatchlistSlabs(itemId);
    try {
      const languagePreference = read(item, 'languagePreference', 'language_preference') || 'BOTH';
      await refreshSlabObservations(item);
      const { data } = await api.get(`/cards/${encodeURIComponent(externalCardId)}/slab-summary?languagePreference=${encodeURIComponent(languagePreference)}`);
      setWatchlistSlabs(current => ({ ...current, [itemId]: data.summary || [] }));
    } catch {
      setWatchlistSlabs(current => ({ ...current, [itemId]: [] }));
    } finally {
      setLoadingWatchlistSlabs(null);
    }
  }

  function updateDiscount(value) {
    setForm(f => {
      const market = effectiveMarketPrice(selectedCard);
      const next = { ...f, targetDiscountPct: value };
      const discount = Number(value);
      if (Number.isFinite(market) && Number.isFinite(discount)) {
        next.targetBuyPrice = (market * (1 - discount / 100)).toFixed(2);
      }
      return next;
    });
  }

  async function handleAdd(e) {
    e.preventDefault();
    if (!selectedCard && !form.query.trim()) {
      alert('Search a card or type a card name first');
      return;
    }
    setSubmitting(true);
    try {
      const payload = {
        cardName: selectedCard?.name || form.query.trim(),
        setName: selectedCard?.set || '',
        externalCardId: selectedCard?.id || '',
        cardNumber: selectedCard?.number || '',
        rarity: selectedCard?.rarity || '',
        imageUrl: selectedCard?.image || '',
        marketPrice: effectiveMarketPrice(selectedCard),
        marketUpdatedAt: selectedCard?.tcgplayerUpdatedAt || selectedCard?.cardmarketUpdatedAt || '',
        targetDiscountPct: selectedCard ? toNumberOrNull(form.targetDiscountPct) : null,
        priceSource: 'poketcg',
        targetBuyPrice: toNumberOrNull(form.targetBuyPrice),
        targetSellPrice: toNumberOrNull(form.targetSellPrice),
        notes: form.notes,
        assetType: form.assetType,
        languagePreference: form.languagePreference,
        slabTier: form.assetType === 'SLAB' ? form.slabTier : '',
        grader: form.assetType === 'SLAB' ? slabParts(form.slabTier).grader : '',
        grade: form.assetType === 'SLAB' ? slabParts(form.slabTier).grade : '',
      };

      const { data } = await api.post('/watchlist', payload);
      dispatch(addWatchlistItem(data.item));
      if (data.item?.externalCardId || data.item?.external_card_id) {
        refreshSlabObservations(data.item).catch(() => {});
      }
      setShowAdd(false);
      resetAddForm();
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

  function renderSlabSummaryRows(rows) {
    return rows.map(row => {
      const listings = row.listings || [];
      return (
        <tr key={row.slabTier}>
          <td><span className={row.slabTier === 'RAW' ? 'badge' : 'badge badge-gold'}>{row.label}</span></td>
          <td>{row.lowestPrice ? <span className="price-tag">{money(row.lowestPrice)}</span> : <span style={{ color: 'var(--color-text-muted)' }}>No observations</span>}</td>
          <td>{row.count || 0}</td>
          <td style={{ minWidth: 260 }}>
            {listings.length > 0 ? (
              <div style={{ display: 'grid', gap: 6 }}>
                {listings.map((listing, index) => (
                  <a key={listing.listingId || `${row.slabTier}-${index}`} href={listing.listingUrl} target="_blank" rel="noreferrer" style={{ display: 'flex', justifyContent: 'space-between', gap: 10, fontSize: '0.75rem' }}>
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{listing.listingTitle || `Listing ${index + 1}`}</span>
                    <strong>{money(listing.price)}</strong>
                  </a>
                ))}
              </div>
            ) : (
              <span style={{ color: 'var(--color-text-muted)' }}>-</span>
            )}
          </td>
        </tr>
      );
    });
  }

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 60 }}><div className="spinner" /></div>;

  return (
    <div>
      <div className="section-header">
        <div className="section-title"><Star size={18} className="icon" /> My Watchlist ({items.length})</div>
        <button className="btn btn-primary" type="button" onClick={() => setShowAdd(s => !s)}><Plus size={16} /> Add Card</button>
      </div>

      {showAdd && (
        <div className="glass-card" style={{ padding: 24, marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'center', marginBottom: 20 }}>
            <h3 style={{ fontWeight: 700 }}>Add Market Watch</h3>
            <button className="btn btn-icon btn-secondary" type="button" onClick={() => { setShowAdd(false); resetAddForm(); }} title="Close"><X size={16} /></button>
          </div>

          <form onSubmit={handleMarketSearch} style={{ marginBottom: 18 }}>
            <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end' }}>
              <div className="form-group" style={{ flex: 1, marginBottom: 0 }}>
                <label className="label">Find Card</label>
                <input className="input" value={form.query} onChange={e => setForm(f => ({ ...f, query: e.target.value }))} placeholder="Radiant Charizard, Pikachu, Umbreon..." />
              </div>
              <button className="btn btn-secondary" type="submit" disabled={searching || !form.query.trim()}><Search size={16} /> {searching ? 'Searching...' : 'Search'}</button>
            </div>
          </form>

          {!searching && form.query && results.length === 0 && (
            <div style={{ padding: 12, marginBottom: 18, border: '1px solid var(--color-border)', borderRadius: 8, color: 'var(--color-text-muted)' }}>
              No card selected yet. Search a card name, then click the exact card/set you want to watch.
            </div>
          )}

          {results.length > 0 && (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 12, marginBottom: 20 }}>
              {results.map(card => {
                const active = selectedCard?.id === card.id;
                return (
                  <button
                    key={card.id}
                    type="button"
                    onClick={() => selectMarketCard(card)}
                    className="glass-card"
                    style={{
                      padding: 12,
                      textAlign: 'left',
                      border: active ? '1px solid var(--color-accent-primary)' : '1px solid var(--color-border)',
                      cursor: 'pointer',
                      display: 'flex',
                      gap: 12,
                      alignItems: 'center',
                    }}
                  >
                    {card.image && <img src={card.image} alt={card.name} style={{ width: 44, height: 62, objectFit: 'cover', borderRadius: 6 }} />}
                    <span style={{ minWidth: 0 }}>
                      <span style={{ display: 'block', fontWeight: 700, fontSize: '0.875rem' }}>{card.name}</span>
                      <span style={{ display: 'block', color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>{card.set} #{card.number}</span>
                      <span className="price-tag" style={{ display: 'block', fontSize: '0.9rem', marginTop: 4 }}>{money(effectiveMarketPrice(card))}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          )}

          <form onSubmit={handleAdd}>
            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 18 }}>
              <button
                className={`btn ${form.assetType === 'RAW' ? 'btn-primary' : 'btn-secondary'}`}
                type="button"
                onClick={() => setForm(f => ({ ...f, assetType: 'RAW' }))}
              >
                Raw Card
              </button>
              <button
                className={`btn ${form.assetType === 'SLAB' ? 'btn-primary' : 'btn-secondary'}`}
                type="button"
                onClick={() => setForm(f => ({ ...f, assetType: 'SLAB' }))}
              >
                Graded Slab
              </button>
              <button
                className={`btn ${form.assetType === 'ALL_SLABS' ? 'btn-primary' : 'btn-secondary'}`}
                type="button"
                onClick={() => setForm(f => ({ ...f, assetType: 'ALL_SLABS' }))}
              >
                Track All Major Slabs
              </button>
            </div>

            {form.assetType === 'SLAB' && (
              <div className="form-group">
                <label className="label">Slab Grade</label>
                <select className="input" value={form.slabTier} onChange={e => setForm(f => ({ ...f, slabTier: e.target.value }))}>
                  {slabOptions.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
              </div>
            )}

            <div className="form-group">
              <label className="label">Language</label>
              <select className="input" value={form.languagePreference} onChange={e => setForm(f => ({ ...f, languagePreference: e.target.value }))}>
                {languageOptions.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
            </div>

            <div style={{ padding: 16, marginBottom: 18, border: '1px solid var(--color-border)', borderRadius: 8, display: 'flex', gap: 14, alignItems: 'center' }}>
              {selectedCard?.image && <img src={selectedCard.image} alt={selectedCard.name} style={{ width: 52, height: 74, objectFit: 'cover', borderRadius: 6 }} />}
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem', marginBottom: 4 }}>Selected Card</div>
                <div style={{ fontWeight: 700 }}>{selectedCard ? selectedCard.name : 'None selected'}</div>
                <div style={{ color: 'var(--color-text-secondary)', fontSize: '0.8125rem' }}>{selectedCard ? `${selectedCard.set} #${selectedCard.number}` : 'Search and click a result above'}</div>
                <div className="badge" style={{ marginTop: 6 }}>{languageOptions.find(option => option.value === form.languagePreference)?.label}</div>
                {form.assetType === 'SLAB' && <div className="badge badge-gold" style={{ marginTop: 6 }}>{slabOptions.find(option => option.value === form.slabTier)?.label}</div>}
                {form.assetType === 'ALL_SLABS' && <div className="badge badge-gold" style={{ marginTop: 6 }}>All major slabs</div>}
                {selectedCard && form.assetType !== 'ALL_SLABS' && (
                  <button className="btn btn-secondary" type="button" onClick={searchLiveListings} disabled={loadingLiveListings} style={{ marginTop: 10 }}>
                    <Search size={15} /> {loadingLiveListings ? 'Checking eBay...' : 'Live eBay'}
                  </button>
                )}
              </div>
              <div style={{ textAlign: 'right' }}>
                <div style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem', marginBottom: 4 }}>Market</div>
                <div className="price-tag" style={{ fontSize: '1rem' }}><DollarSign size={15} /> {selectedCard ? money(effectiveMarketPrice(selectedCard)) : '-'}</div>
              </div>
            </div>

            {liveListings.length > 0 && (
              <div style={{ marginBottom: 18, border: '1px solid var(--color-border)', borderRadius: 8, overflow: 'hidden' }}>
                <div style={{ padding: '12px 14px', background: 'var(--color-bg-elevated)', display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center' }}>
                  <strong style={{ fontSize: '0.875rem' }}>Live eBay Matches</strong>
                  <span style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>{liveListings.length} active</span>
                </div>
                <div style={{ display: 'grid', gap: 8, padding: 14 }}>
                  {liveListings.slice(0, 25).map((listing, index) => (
                    <a key={listing.listing_id || index} href={listing.listing_url} target="_blank" rel="noreferrer" style={{ display: 'grid', gridTemplateColumns: '64px 1fr auto', gap: 12, alignItems: 'center' }}>
                      {listing.image_url ? <img src={listing.image_url} alt="" style={{ width: 48, height: 48, objectFit: 'cover', borderRadius: 6 }} /> : <span />}
                      <span style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: '0.8125rem' }}>{listing.listing_title || listing.listing_id}</span>
                      <strong className="price-tag">{money(listing.price)}</strong>
                    </a>
                  ))}
                </div>
              </div>
            )}

            {selectedCard && (
              <div style={{ marginBottom: 18, border: '1px solid var(--color-border)', borderRadius: 8, overflow: 'hidden' }}>
                <div style={{ padding: '12px 14px', background: 'var(--color-bg-elevated)', display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center' }}>
                  <strong style={{ fontSize: '0.875rem' }}>Observed eBay Slabs</strong>
                  <span style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>{loadingSlabs ? 'Loading...' : 'Last 7 days'}</span>
                </div>
                <div style={{ overflowX: 'auto' }}>
                  <table className="data-table">
                    <thead>
                      <tr><th>Lane</th><th>Lowest Active</th><th>Listings</th><th>Matches</th></tr>
                    </thead>
                    <tbody>
                      {renderSlabSummaryRows(visibleSummaryRows)}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            <div className="grid-3">
              <div className="form-group">
                <label className="label">Below Market</label>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Percent size={16} color="var(--color-text-muted)" />
                  <input className="input" type="number" min="0" max="95" step="1" value={form.targetDiscountPct} onChange={e => updateDiscount(e.target.value)} />
                </div>
              </div>
              <div className="form-group">
                <label className="label">Buy Alert Price ($)</label>
                <input className="input" type="number" step="0.01" value={form.targetBuyPrice || calculatedTarget} onChange={e => setForm(f => ({ ...f, targetBuyPrice: e.target.value }))} placeholder="Alert when listing is at or below..." />
              </div>
              <div className="form-group">
                <label className="label">Sell Alert Price ($)</label>
                <input className="input" type="number" step="0.01" value={form.targetSellPrice} onChange={e => setForm(f => ({ ...f, targetSellPrice: e.target.value }))} placeholder="Optional upside alert" />
              </div>
            </div>

            <div className="form-group">
              <label className="label">Notes</label>
              <input className="input" value={form.notes} onChange={e => setForm(f => ({ ...f, notes: e.target.value }))} placeholder="Condition, grading plan, max fees, or seller notes" />
            </div>
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
              <button className="btn btn-primary" type="submit" disabled={submitting || (!selectedCard && !form.query.trim())}>{submitting ? 'Adding...' : 'Add to Watchlist'}</button>
              <button className="btn btn-secondary" type="button" onClick={resetAddForm}>Clear</button>
            </div>
          </form>
        </div>
      )}

      {items.length > 0 ? (
        <div className="glass-card" style={{ overflow: 'hidden' }}>
          <table className="data-table">
            <thead>
              <tr>
                <th>Card</th><th>Market</th><th>Buy Alert</th><th>Sell Alert</th><th>Notes</th><th>Added</th><th></th>
              </tr>
            </thead>
            <tbody>
              {items.map(item => {
                const cardName = read(item, 'cardName', 'card_name');
                const setName = read(item, 'setName', 'set_name');
                const imageUrl = read(item, 'imageUrl', 'image_url');
                const marketPrice = read(item, 'marketPrice', 'market_price');
                const discount = read(item, 'targetDiscountPct', 'target_discount_pct');
                const buyPrice = read(item, 'targetBuyPrice', 'target_buy_price');
                const sellPrice = read(item, 'targetSellPrice', 'target_sell_price');
                const notes = read(item, 'notes');
                const addedAt = read(item, 'addedAt', 'added_at');
                const assetType = read(item, 'assetType', 'asset_type') || 'RAW';
                const slabTier = read(item, 'slabTier', 'slab_tier');
                const languagePreference = read(item, 'languagePreference', 'language_preference') || 'BOTH';
                const slabLabel = slabOptions.find(option => option.value === slabTier)?.label;
                const itemId = read(item, 'id');
                const externalCardId = read(item, 'externalCardId', 'external_card_id');
                const expanded = expandedSlabCard === itemId;
                const rowSummary = summaryRowsFromMap(new Map((watchlistSlabs[itemId] || []).map(row => [row.slabTier || row.slab_tier, row])));
                return [
                  <tr key={`${itemId}-main`}>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        {imageUrl && <img src={imageUrl} alt={cardName} style={{ width: 34, height: 48, objectFit: 'cover', borderRadius: 5 }} />}
                        <div>
                          <strong>{cardName}</strong>
                          <div style={{ color: 'var(--color-text-secondary)', fontSize: '0.75rem' }}>{setName || '-'}</div>
                          <span className="badge" style={{ marginTop: 4 }}>{languageOptions.find(option => option.value === languagePreference)?.label || 'English + Japanese'}</span>
                          {assetType === 'SLAB' && <span className="badge badge-gold" style={{ marginTop: 4 }}>{slabLabel || 'Graded Slab'}</span>}
                          {assetType === 'ALL_SLABS' && <span className="badge badge-gold" style={{ marginTop: 4 }}>All major slabs</span>}
                        </div>
                      </div>
                    </td>
                    <td>{marketPrice ? <span className="price-tag" style={{ fontSize: '0.95rem' }}>{money(marketPrice)}</span> : <span style={{ color: 'var(--color-text-muted)' }}>Manual</span>}</td>
                    <td>{buyPrice ? <span className="badge badge-rising">≤ {money(buyPrice)} {discount ? `(${pct(discount)} under)` : ''}</span> : <span style={{ color: 'var(--color-text-muted)' }}>-</span>}</td>
                    <td>{sellPrice ? <span className="badge badge-falling">≥ {money(sellPrice)}</span> : <span style={{ color: 'var(--color-text-muted)' }}>-</span>}</td>
                    <td style={{ color: 'var(--color-text-muted)', fontSize: '0.8125rem' }}>{notes || '-'}</td>
                    <td style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>{addedAt ? new Date(addedAt).toLocaleDateString() : '-'}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                        <button className="btn btn-secondary" type="button" onClick={() => toggleWatchlistSlabs(item)} disabled={!externalCardId} title={externalCardId ? 'View observed slab prices' : 'PokeTCG card ID required'}>
                          <Search size={15} /> Slabs
                        </button>
                        <button className="btn btn-icon btn-secondary" onClick={() => handleRemove(item.id, cardName)} title="Remove from watchlist"><Trash2 size={15} color="var(--color-accent-falling)" /></button>
                      </div>
                    </td>
                  </tr>,
                  expanded && (
                    <tr key={`${itemId}-slabs`}>
                      <td colSpan="7" style={{ padding: 0, background: 'var(--color-bg-elevated)' }}>
                        <div style={{ padding: 16 }}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', marginBottom: 10 }}>
                            <strong style={{ fontSize: '0.875rem' }}>Observed eBay Slabs</strong>
                            <span style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>{loadingWatchlistSlabs === itemId ? 'Scanning eBay live...' : 'Live eBay + last 7 days'}</span>
                          </div>
                          <div style={{ overflowX: 'auto' }}>
                            <table className="data-table">
                              <thead>
                                <tr><th>Lane</th><th>Lowest Active</th><th>Listings</th><th>Matches</th></tr>
                              </thead>
                              <tbody>
                                {renderSlabSummaryRows(rowSummary)}
                              </tbody>
                            </table>
                          </div>
                          <div style={{ marginTop: 14, display: 'grid', gap: 8 }}>
                            <textarea
                              className="input"
                              value={importTextByItem[itemId] || ''}
                              onChange={e => setImportTextByItem(current => ({ ...current, [itemId]: e.target.value }))}
                              placeholder="Paste copied eBay search results for this exact card here"
                              rows={5}
                              style={{ resize: 'vertical', minHeight: 96 }}
                            />
                            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
                              <span style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>Imported rows are filtered by card name, set, number, language, and slab grade before saving.</span>
                              <button className="btn btn-secondary" type="button" onClick={() => importEbayText(item)} disabled={importingItemId === itemId || !(importTextByItem[itemId] || '').trim()}>
                                <Plus size={15} /> {importingItemId === itemId ? 'Importing...' : 'Import eBay Text'}
                              </button>
                            </div>
                          </div>
                        </div>
                      </td>
                    </tr>
                  ),
                ];
              })}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="glass-card empty-state" style={{ padding: 60 }}>
          <Bell size={48} />
          <h3 style={{ fontWeight: 700 }}>Your watchlist is empty</h3>
          <p>Search market prices and add cards to start receiving buy alerts</p>
          <button className="btn btn-primary" onClick={() => setShowAdd(true)}><Plus size={16} /> Add Your First Card</button>
        </div>
      )}
    </div>
  );
}
