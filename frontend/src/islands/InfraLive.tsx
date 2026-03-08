import { useState, useEffect, useRef } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface InfraMetrics {
  uptime: string;
  mem_total: string;
  mem_available: string;
  mem_used_pct: number;
  load_avg: string;
  requests_min: number;
  timestamp: string;
}

interface Props {
  labels: {
    uptime: string;
    latency: string;
    requestsPerMinute: string;
    memoryUsage: string;
    loadAvg: string;
    memTotal: string;
    memAvailable: string;
    live: string;
    connecting: string;
  };
}

const defaultMetrics: InfraMetrics = {
  uptime: '--',
  mem_total: '--',
  mem_available: '--',
  mem_used_pct: 0,
  load_avg: '--',
  requests_min: 0,
  timestamp: '',
};

const MAX_SSE_RETRIES = 2;

const InfraLive: FunctionComponent<Props> = ({ labels }) => {
  const [metrics, setMetrics] = useState<InfraMetrics>(defaultMetrics);
  const [connected, setConnected] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout>>();
  const pollIntervalRef = useRef<ReturnType<typeof setInterval>>();
  const sseRetriesRef = useRef(0);

  const fallbackFetch = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/infra`);
      if (res.ok) {
        const data: InfraMetrics = await res.json();
        setMetrics(data);
        setConnected(true);
      }
    } catch {
      // Silently fail
    }
  };

  const startPolling = () => {
    fallbackFetch();
    pollIntervalRef.current = setInterval(fallbackFetch, 10000);
  };

  const connectSSE = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    try {
      const es = new EventSource(`${API_BASE}/api/infra/stream`);
      eventSourceRef.current = es;

      es.onopen = () => {
        setConnected(true);
        sseRetriesRef.current = 0;
      };

      es.addEventListener('metrics', (event) => {
        try {
          const data: InfraMetrics = JSON.parse(event.data);
          setMetrics(data);
        } catch {
          // Invalid JSON, skip
        }
      });

      es.onerror = () => {
        es.close();
        setConnected(false);
        sseRetriesRef.current++;

        if (sseRetriesRef.current >= MAX_SSE_RETRIES) {
          // SSE not working (e.g. HTTP/3/QUIC), fall back to polling
          startPolling();
        } else {
          reconnectTimeoutRef.current = setTimeout(connectSSE, 3000);
        }
      };
    } catch {
      // SSE constructor failed, fall back to polling
      startPolling();
    }
  };

  useEffect(() => {
    if (typeof EventSource !== 'undefined') {
      connectSSE();
    } else {
      startPolling();
    }

    return () => {
      eventSourceRef.current?.close();
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
      if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
    };
  }, []);

  const memDisplay = metrics.mem_used_pct > 0
    ? `${metrics.mem_used_pct.toFixed(1)}%`
    : '--';

  const metricItems = [
    { label: labels.uptime, value: metrics.uptime, unit: '' },
    { label: labels.requestsPerMinute, value: String(metrics.requests_min), unit: '/min' },
    { label: labels.memoryUsage, value: memDisplay, unit: '' },
    { label: labels.loadAvg, value: metrics.load_avg, unit: '' },
    { label: labels.memTotal, value: metrics.mem_total, unit: '' },
    { label: labels.memAvailable, value: metrics.mem_available, unit: '' },
  ];

  return (
    <div class="infra-live" aria-live="polite">
      <div class="infra-status">
        <span class={`freshness-dot ${connected ? 'freshness-active' : 'freshness-archived'}`} aria-hidden="true" />
        <span class="infra-status-text" style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-muted)' }}>
          {connected ? labels.live : labels.connecting}
        </span>
        {metrics.timestamp && (
          <span class="infra-timestamp" style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)', color: 'var(--color-text-muted)', marginLeft: 'var(--space-2)' }}>
            {new Date(metrics.timestamp).toLocaleTimeString()}
          </span>
        )}
      </div>

      <div class="infra-grid">
        {metricItems.map(item => (
          <div class="infra-metric" key={item.label}>
            <div class="infra-metric-label">{item.label}</div>
            <div class="infra-metric-value">
              {item.value}{item.unit}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default InfraLive;
