import { useEffect, useMemo, useState } from 'react';
import { ExternalLink, Filter, RefreshCw, ShieldCheck, ThumbsDown, Trophy, Zap } from 'lucide-react';
import api from '../services/api.js';

const graders = ['', 'PSA', 'CGC', 'BGS'];
const tiers = ['', 'PSA_10', 'PSA_9', 'CGC_10', 'CGC_9_5', 'BGS_10', 'BGS_BLACK_LABEL'];

function money(value) {
  const number = Number(value || 0);
  return `$${number.toFixed(2)}`;
}

function scoreColor(score) {
  if (score >= 220) return 'var(--color-accent-rising)';
  if (score >= 140) return 'var(--color-accent-gold)';
  return 'var(--color-accent-warning)';
}

function evidenceOf(item) {
  if (!item?.evidence) return {};
  if (typeof item.evidence === 'object') return item.evidence;
  try {
    return JSON.parse(item.evidence);
  } catch {
    return {};
  }
}

function sellerHubOf(item) {
  if (!item?.sellerHubMetrics) return { active: null, sold: null };
  if (typeof item.sellerHubMetrics === 'object') return item.sellerHubMetrics;
  try {
    return JSON.parse(item.sellerHubMetrics);
  } catch {
    return { active: null, sold: null };
  }
}

function compactNumber(value, suffix = '') {
  if (value === null || value === undefined || value === '') return '-';
  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  return `${Number.isInteger(number) ? number : number.toFixed(1)}${suffix}`;
}

function shortDate(value) {
  if (!value) return '';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '';
  return parsed.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function signalLabel(item) {
  const signal = evidenceOf(item).signal;
  if (signal === 'BUY_CANDIDATE') return 'Buy candidate';
  if (signal === 'SELL_RESEARCH') return 'Sell research';
  if (signal === 'RESEARCH_TARGET') return 'Research target';
  return item.decision || 'Signal';
}

function signalColor(item) {
  const signal = evidenceOf(item).signal;
  if (signal === 'BUY_CANDIDATE') return 'var(--color-accent-rising)';
  if (signal === 'SELL_RESEARCH') return 'var(--color-accent-gold)';
  if (signal === 'RESEARCH_TARGET') return 'var(--color-accent-info, #38bdf8)';
  return scoreColor(item.dealScore);
}

export default function SlabOpportunitiesPage() {
  const [opportunities, setOpportunities] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filters, setFilters] = useState({ decision: '', signal: '', grader: '', slabTier: '', minMarginPct: '', q: '' });
  const [busyId, setBusyId] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  const [refreshSummary, setRefreshSummary] = useState(null);

  const query = useMemo(() => {
    const params = new URLSearchParams();
    Object.entries(filters).forEach(([key, value]) => {
      if (value !== '') params.set(key, value);
    });
    params.set('limit', '50');
    return params.toString();
  }, [filters]);

  async function load() {
    setLoading(true);
    setError('');
    try {
      const { data } = await api.get(`/slab-opportunities?${query}`);
      setOpportunities(data.opportunities || []);
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to load slab opportunities');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [query]);

  async function refreshLiveFinder() {
    setRefreshing(true);
    setError('');
    try {
      const { data } = await api.post('/slab-opportunities/refresh-live');
      setRefreshSummary(data);
      await load();
    } catch (err) {
      setError(err.response?.data?.error || 'Failed to refresh live finder');
    } finally {
      setRefreshing(false);
    }
  }

  async function decide(id, action) {
    setBusyId(id);
    try {
      await api.post(`/slab-opportunities/${id}/${action}`);
      await load();
    } catch (err) {
      setError(err.response?.data?.error || `Failed to ${action} opportunity`);
    } finally {
      setBusyId('');
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div className="glass-card" style={{ padding: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'flex-start', flexWrap: 'wrap' }}>
          <div>
            <div className="section-title"><Trophy size={20} className="icon" /> Finder</div>
            <p style={{ color: 'var(--color-text-muted)', margin: '8px 0 0', maxWidth: 780 }}>
              Market-discovered slab recommendations from Scrapling/PriceCharting movers plus live eBay asks. This is independent of your watchlist and shows the target buy price needed for a real flip.
            </p>
            {refreshSummary && (
              <p style={{ color: 'var(--color-text-muted)', margin: '8px 0 0', fontSize: '0.78rem' }}>
                Last refresh: {refreshSummary.persisted || 0} persisted, {refreshSummary.buyCandidates || 0} buys, {refreshSummary.sellResearch || 0} sell-research, {refreshSummary.researchTargets || 0} research targets.
              </p>
            )}
          </div>
          <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
            <button className="btn btn-secondary btn-sm" type="button" onClick={refreshLiveFinder} disabled={refreshing}>
              <RefreshCw size={14} /> {refreshing ? 'Refreshing...' : 'Refresh live'}
            </button>
            <div className="badge badge-gold" style={{ alignSelf: 'center' }}><Zap size={14} /> Human approval required</div>
          </div>
        </div>
      </div>

      <div className="glass-card" style={{ padding: 18 }}>
        <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr 1fr', gap: 12, alignItems: 'end' }}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
            Card search
            <input className="input" value={filters.q} onChange={e => setFilters(f => ({ ...f, q: e.target.value }))} placeholder="Charizard, Mewtwo..." />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
            Grader
            <select className="input" value={filters.grader} onChange={e => setFilters(f => ({ ...f, grader: e.target.value }))}>
              {graders.map(value => <option key={value} value={value}>{value || 'Any'}</option>)}
            </select>
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
            Tier
            <select className="input" value={filters.slabTier} onChange={e => setFilters(f => ({ ...f, slabTier: e.target.value }))}>
              {tiers.map(value => <option key={value} value={value}>{value || 'Any'}</option>)}
            </select>
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
            Min margin %
            <input className="input" type="number" value={filters.minMarginPct} onChange={e => setFilters(f => ({ ...f, minMarginPct: e.target.value }))} />
          </label>

          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
            Signal
            <select className="input" value={filters.signal} onChange={e => setFilters(f => ({ ...f, signal: e.target.value }))}>
              <option value="">All</option>
              <option value="BUY_CANDIDATE">Buy candidates</option>
              <option value="SELL_RESEARCH">Sell research</option>
              <option value="RESEARCH_TARGET">Research targets</option>
            </select>
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
            Decision
            <select className="input" value={filters.decision} onChange={e => setFilters(f => ({ ...f, decision: e.target.value }))}>
              <option value="">All</option>
              <option value="candidate">Candidate</option>
              <option value="watch">Watch</option>
              <option value="approved">Approved</option>
              <option value="rejected">Rejected</option>
            </select>
          </label>
        </div>
      </div>

      {error && <div className="glass-card" style={{ padding: 16, borderColor: 'var(--color-accent-falling)', color: 'var(--color-accent-falling)' }}>{error}</div>}
      {loading && <div style={{ display: 'flex', justifyContent: 'center', paddingTop: 60 }}><div className="spinner" /></div>}

      {!loading && opportunities.length === 0 && (
        <div className="empty-state glass-card" style={{ padding: 48 }}>
          <Filter size={42} />
          <p>No slab opportunities match these filters yet. Click Refresh live to scan PriceCharting movers and eBay asks, then Finder will rank what it finds.</p>
        </div>
      )}

      {!loading && opportunities.map(item => (
        <div key={item.id} className="glass-card" style={{ padding: 20, display: 'grid', gridTemplateColumns: '1.4fr 1fr auto', gap: 18, alignItems: 'center' }}>
          <div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', marginBottom: 8 }}>
              <span className="badge badge-gold">{item.slabTier || 'Slab'}</span>
              <span className="badge">{item.marketplace || 'marketplace'}</span>
              <span className="badge" style={{ color: signalColor(item) }}>{signalLabel(item)}</span>
              <span className="badge" style={{ color: scoreColor(item.dealScore) }}>Score {item.dealScore}</span>
            </div>
            <div style={{ fontWeight: 800, fontSize: '1rem', marginBottom: 4 }}>{item.cardName}</div>
            <div style={{ color: 'var(--color-text-muted)', fontSize: '0.8rem', marginBottom: 8 }}>{item.title || item.setName || 'No listing title captured'}</div>
            <div style={{ color: 'var(--color-text-secondary)', fontSize: '0.8rem' }}>{item.reason}</div>
            {evidenceOf(item).referenceUrl && (
              <a href={evidenceOf(item).referenceUrl} target="_blank" rel="noreferrer" style={{ display: 'inline-flex', marginTop: 8, color: 'var(--color-accent-gold)', fontSize: '0.78rem' }}>Reference source</a>
            )}
            <SellerHubStrip metrics={sellerHubOf(item)} />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
            <Metric label="Ask" value={money(item.askingPrice)} />
            <Metric label="Target buy" value={money(evidenceOf(item).targetBuyPrice)} strong />
            <Metric label="All-in cost" value={money(item.allInCost)} />
            <Metric label="Market value" value={money(item.estimatedMarketValue)} />
            <Metric label="Profit" value={money(item.expectedProfit)} strong />
            <Metric label="Margin" value={`${Number(item.expectedMarginPct || 0).toFixed(1)}%`} strong />
            <Metric label="Confidence" value={`${item.confidenceScore}/100`} />
            <Metric label="Risk" value={`${item.riskScore}/100`} />
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, minWidth: 150 }}>
            {item.listingUrl && <a className="btn btn-secondary btn-sm" href={item.listingUrl} target="_blank" rel="noreferrer"><ExternalLink size={14} /> {evidenceOf(item).needsEbayScan ? 'Search eBay' : 'Listing'}</a>}
            <button className="btn btn-primary btn-sm" disabled={busyId === item.id || item.decision === 'approved' || evidenceOf(item).needsEbayScan} onClick={() => decide(item.id, 'approve')} title={evidenceOf(item).needsEbayScan ? 'Find a real eBay listing before approving' : ''}>
              <ShieldCheck size={14} /> Approve
            </button>
            <button className="btn btn-secondary btn-sm" disabled={busyId === item.id || item.decision === 'rejected'} onClick={() => decide(item.id, 'reject')}>
              <ThumbsDown size={14} /> Reject
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}

function SellerHubStrip({ metrics }) {
  const active = metrics?.active;
  const sold = metrics?.sold;
  if (!active && !sold) return null;
  return (
    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 10 }}>
      {active && (
        <ResearchPill
          label="eBay active"
          href={active.sourceUrl}
          values={[`${compactNumber(active.totalListings)} listings`, money(active.avgListingPrice), `${compactNumber(active.avgWatchers)} avg watchers`, `${compactNumber(active.maxWatchers)} max watchers`]}
          date={shortDate(active.researchedAt)}
        />
      )}
      {sold && (
        <ResearchPill
          label="eBay sold"
          href={sold.sourceUrl}
          values={[`${compactNumber(sold.totalListings)} sold`, money(sold.avgListingPrice), `${compactNumber(sold.avgBids)} avg bids`, `${compactNumber(sold.maxBids)} max bids`]}
          date={shortDate(sold.researchedAt)}
        />
      )}
    </div>
  );
}

function ResearchPill({ label, values, href, date }) {
  const body = (
    <>
      <span style={{ fontWeight: 800, color: 'var(--color-text-primary)' }}>{label}</span>
      {values.map(value => <span key={value}>{value}</span>)}
      {date && <span>{date}</span>}
    </>
  );
  const style = {
    display: 'inline-flex',
    gap: 8,
    alignItems: 'center',
    flexWrap: 'wrap',
    border: '1px solid var(--color-border)',
    borderRadius: 8,
    padding: '8px 10px',
    color: 'var(--color-text-muted)',
    fontSize: '0.72rem',
    textDecoration: 'none',
    background: 'var(--color-bg-elevated)',
  };
  if (!href) return <div style={style}>{body}</div>;
  return <a href={href} target="_blank" rel="noreferrer" style={style}>{body}<ExternalLink size={12} /></a>;
}


function Metric({ label, value, strong = false }) {
  return (
    <div style={{ background: 'var(--color-bg-elevated)', border: '1px solid var(--color-border)', borderRadius: 8, padding: 10 }}>
      <div style={{ fontSize: '0.68rem', color: 'var(--color-text-muted)', marginBottom: 4 }}>{label}</div>
      <div style={{ fontWeight: strong ? 800 : 650, color: strong ? 'var(--color-accent-rising)' : 'var(--color-text-primary)' }}>{value}</div>
    </div>
  );
}
