// ============================================================
// PokémonTool — SSE (Server-Sent Events) Client
// Connects to the Node.js /api/stream endpoint and dispatches
// incoming events to the Redux store for real-time UI updates.
// ============================================================

import { store } from '../store/index.js';
import { addAlert } from '../store/index.js';

let eventSource = null;

/**
 * Opens a persistent SSE connection to the backend stream endpoint.
 * Automatically dispatches incoming events to the Redux store.
 * Call this once after the user logs in.
 */
export function connectSSE() {
  if (eventSource) return; // Already connected

  const token = localStorage.getItem('pt_token');
  if (!token) return;

  // EventSource doesn't support custom headers, so we pass the token as a query param.
  // The server reads it from req.query.token on the /api/stream route.
  // NOTE: For true security, use a short-lived stream token issued by your auth service.
  eventSource = new EventSource(`/api/stream?token=${token}`);

  eventSource.onopen = () => {
    console.log('[SSE] Stream connected ✓');
  };

  // Listen for any named or unnamed events
  eventSource.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      handleSSEEvent(data);
    } catch (err) {
      console.warn('[SSE] Could not parse event:', event.data);
    }
  };

  eventSource.onerror = (err) => {
    console.warn('[SSE] Stream error — will auto-reconnect');
    // EventSource automatically reconnects with exponential backoff
  };
}

/**
 * Dispatches an incoming SSE event to the appropriate Redux action.
 * @param {object} data - The parsed JSON event payload
 */
function handleSSEEvent(data) {
  const { type } = data;

  switch (type) {
    // A watchlist card dropped below buy target
    case 'PRICE_DROP':
    case 'PRICE_SPIKE':
    case 'WATCHLIST_HIT':
    case 'TREND_UPDATE':
    case 'DEAL_FOUND': {
      // Format into an alert item and add to the Redux store
      store.dispatch(addAlert({
        id:          crypto.randomUUID(),
        alertType:   type,
        cardName:    data.cardName,
        message:     data.message,
        price:       data.price,
        marketplace: data.marketplace,
        listingUrl:  data.listingUrl,
        listingId:   data.listingId,
        isRead:      false,
        createdAt:   data.timestamp || new Date().toISOString(),
      }));
      break;
    }
    case 'CONNECTED':
      // Heartbeat / connection confirmation — no action needed
      break;
    default:
      console.log('[SSE] Unknown event type:', type, data);
  }
}

/**
 * Closes the SSE connection (call on logout).
 */
export function disconnectSSE() {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
    console.log('[SSE] Stream disconnected');
  }
}
