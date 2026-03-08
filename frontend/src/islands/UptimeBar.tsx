import { useState, useEffect } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface UptimeDay {
  date: string;
  checks_ok: number;
  checks_total: number;
  uptime_pct: number;
  status: string; // "up", "partial", "down", "unknown"
}

interface Props {
  labels: {
    title: string;
    daysAgo: string;
    today: string;
    fullUptime: string;
    partialOutage: string;
    noData: string;
  };
}

const UptimeBar: FunctionComponent<Props> = ({ labels }) => {
  const [days, setDays] = useState<UptimeDay[]>([]);

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch(`${API_BASE}/api/infra/uptime`);
        if (res.ok) {
          const data: UptimeDay[] = await res.json();
          setDays(data);
        }
      } catch {
        // Silently fail
      }
    })();
  }, []);

  const getStatusClass = (status: string) => {
    switch (status) {
      case 'up': return 'uptime-day-up';
      case 'partial': return 'uptime-day-partial';
      case 'down': return 'uptime-day-down';
      default: return 'uptime-day-unknown';
    }
  };

  const getTooltip = (day: UptimeDay) => {
    const date = new Date(day.date + 'T00:00:00').toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
    if (day.status === 'unknown') return `${date}: ${labels.noData}`;
    if (day.status === 'up') return `${date}: ${labels.fullUptime}`;
    if (day.status === 'partial') return `${date}: ${labels.partialOutage} (${day.uptime_pct.toFixed(1)}%)`;
    return `${date}: Down`;
  };

  if (days.length === 0) {
    return (
      <div>
        <h2>{labels.title}</h2>
        <div class="uptime-bar" style={{ marginTop: 'var(--space-4)' }}>
          {Array.from({ length: 30 }, (_, i) => (
            <div class="uptime-day uptime-day-unknown" key={i} title={labels.noData} aria-label={labels.noData} />
          ))}
        </div>
        <div class="uptime-labels">
          <span class="text-muted font-mono" style={{ fontSize: 'var(--font-size-xs)' }}>{labels.daysAgo}</span>
          <span class="text-muted font-mono" style={{ fontSize: 'var(--font-size-xs)' }}>{labels.today}</span>
        </div>
      </div>
    );
  }

  return (
    <div>
      <h2>{labels.title}</h2>
      <div class="uptime-bar" style={{ marginTop: 'var(--space-4)' }}>
        {days.map((day) => (
          <div
            class={`uptime-day ${getStatusClass(day.status)}`}
            key={day.date}
            title={getTooltip(day)}
            aria-label={getTooltip(day)}
          />
        ))}
      </div>
      <div class="uptime-labels">
        <span class="text-muted font-mono" style={{ fontSize: 'var(--font-size-xs)' }}>{labels.daysAgo}</span>
        <span class="text-muted font-mono" style={{ fontSize: 'var(--font-size-xs)' }}>{labels.today}</span>
      </div>
    </div>
  );
};

export default UptimeBar;
