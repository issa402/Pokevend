import { AlertTriangle, Target } from 'lucide-react';

function actionLabel(action) {
  return {
    SOURCE_NOW: 'Source now',
    CONSIDER_BUY: 'Consider buy',
    FIND_EXACT_LISTING: 'Find exact listing',
    REFRESH_RESEARCH: 'Refresh research',
    AVOID: 'Avoid',
    WATCH: 'Watch',
  }[action] || 'Review';
}

export function actionColor(action) {
  if (action === 'SOURCE_NOW') return 'var(--color-accent-rising)';
  if (action === 'CONSIDER_BUY') return 'var(--color-accent-gold)';
  if (action === 'AVOID') return 'var(--color-accent-falling)';
  return 'var(--color-accent-warning)';
}

export function VendorActionBadge({ insight }) {
  return (
    <>
      <span className="badge" style={{ color: actionColor(insight?.action) }}>
        {actionLabel(insight?.action)}
      </span>
      <span className="badge" style={{ color: actionColor(insight?.action) }}>
        Opportunity {insight?.opportunityScore || 0}/100
      </span>
    </>
  );
}

export default function VendorInsightPanel({ insight }) {
  if (!insight?.action) return null;
  const Icon = insight.action === 'AVOID' ? AlertTriangle : Target;
  return (
    <div style={{
      marginTop: 10,
      padding: 10,
      borderRadius: 8,
      border: `1px solid ${actionColor(insight.action)}`,
      background: 'var(--color-bg-elevated)',
      color: 'var(--color-text-secondary)',
      fontSize: '0.78rem',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: actionColor(insight.action), fontWeight: 800, marginBottom: 4 }}>
        <Icon size={14} /> {actionLabel(insight.action)}
      </div>
      <div>{insight.actionReason}</div>
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginTop: 7, color: 'var(--color-text-muted)', fontSize: '0.7rem' }}>
        <span>Demand {insight.demandScore || 0}/25</span>
        <span>Price {insight.priceScore || 0}/20</span>
        <span>Evidence {insight.evidenceScore || 0}/15</span>
        <span>{insight.benchmarkSource || 'Reference value'}</span>
        {insight.sellerHubStale && <span style={{ color: 'var(--color-accent-warning)' }}>Seller Hub data is stale</span>}
      </div>
    </div>
  );
}

