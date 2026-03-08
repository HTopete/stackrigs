import { useState } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface BuildUpdate {
  id: number;
  build_id: number;
  type: string;
  title: string;
  content: string;
  created_at: string;
}

interface Props {
  buildId: string;
  locale: string;
  labels: {
    title: string;
    addUpdate: string;
    updateTitle: string;
    updateTitlePlaceholder: string;
    content: string;
    contentPlaceholder: string;
    type: string;
    submit: string;
    cancel: string;
    saving: string;
    deleteConfirm: string;
    noUpdates: string;
    typeLabels: Record<string, string>;
  };
}

const UPDATE_TYPES = ['milestone', 'stack_change', 'infra_change', 'tool_change', 'reflection', 'pivot'] as const;

const BuildUpdatesIsland: FunctionComponent<Props> = ({ buildId, locale, labels }) => {
  const [updates, setUpdates] = useState<BuildUpdate[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [isOwner, setIsOwner] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ type: 'milestone', title: '', content: '' });

  // Load updates and check ownership on mount
  if (!loaded) {
    setLoaded(true);
    Promise.all([
      fetch(`${API_BASE}/api/builds/${buildId}`).then(r => r.ok ? r.json() : null),
      fetch(`${API_BASE}/api/auth/me`, { credentials: 'include' }).then(r => r.ok ? r.json() : null),
    ]).then(([buildData, meData]) => {
      if (buildData?.updates) setUpdates(buildData.updates);
      if (meData?.builder && buildData && meData.builder.id === buildData.builder_id) {
        setIsOwner(true);
      }
    }).catch(() => {});
  }

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    if (!form.title.trim()) return;
    setSaving(true);

    try {
      const res = await fetch(`${API_BASE}/api/builds/${buildId}/updates`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(form),
      });
      if (res.ok) {
        const update = await res.json();
        setUpdates(prev => [update, ...prev]);
        setForm({ type: 'milestone', title: '', content: '' });
        setShowForm(false);
      }
    } catch {}
    setSaving(false);
  };

  const handleDelete = async (updateId: number) => {
    if (!confirm(labels.deleteConfirm)) return;
    try {
      const res = await fetch(`${API_BASE}/api/builds/${buildId}/updates/${updateId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (res.ok) {
        setUpdates(prev => prev.filter(u => u.id !== updateId));
      }
    } catch {}
  };

  const formatDate = (iso: string) => {
    try {
      return new Date(iso).toLocaleDateString(locale, {
        year: 'numeric', month: 'short', day: 'numeric',
      });
    } catch {
      return iso;
    }
  };

  return (
    <div class="build-updates-section">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-4)' }}>
        <h2 style={{ margin: 0 }}>{labels.title}</h2>
        {isOwner && !showForm && (
          <button class="btn btn-sm btn-primary" onClick={() => setShowForm(true)}>
            {labels.addUpdate}
          </button>
        )}
      </div>

      {showForm && (
        <form class="build-form" onSubmit={handleSubmit} style={{ marginBottom: 'var(--space-6)', padding: 'var(--space-4)', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-md)' }}>
          <div class="build-form-row">
            <div class="build-form-field">
              <label for="bu-type">{labels.type}</label>
              <select
                id="bu-type"
                value={form.type}
                onChange={(e) => setForm(prev => ({ ...prev, type: (e.target as HTMLSelectElement).value }))}
              >
                {UPDATE_TYPES.map(t => (
                  <option key={t} value={t}>{labels.typeLabels[t] || t}</option>
                ))}
              </select>
            </div>
            <div class="build-form-field" style={{ flex: 2 }}>
              <label for="bu-title">{labels.updateTitle} *</label>
              <input
                id="bu-title"
                type="text"
                value={form.title}
                onInput={(e) => setForm(prev => ({ ...prev, title: (e.target as HTMLInputElement).value }))}
                placeholder={labels.updateTitlePlaceholder}
                required
              />
            </div>
          </div>
          <div class="build-form-field">
            <label for="bu-content">{labels.content}</label>
            <textarea
              id="bu-content"
              value={form.content}
              onInput={(e) => setForm(prev => ({ ...prev, content: (e.target as HTMLTextAreaElement).value }))}
              placeholder={labels.contentPlaceholder}
              rows={3}
            />
          </div>
          <div style={{ display: 'flex', gap: 'var(--space-3)' }}>
            <button type="submit" class="btn btn-primary btn-sm" disabled={saving}>
              {saving ? labels.saving : labels.submit}
            </button>
            <button type="button" class="btn btn-sm" onClick={() => setShowForm(false)}>
              {labels.cancel}
            </button>
          </div>
        </form>
      )}

      {updates.length === 0 && !showForm && (
        <p class="text-muted">{labels.noUpdates}</p>
      )}

      <div class="build-updates-timeline">
        {updates.map(update => (
          <div key={update.id} class="build-update-entry" style={{
            borderLeft: '2px solid var(--color-accent)',
            paddingLeft: 'var(--space-4)',
            marginBottom: 'var(--space-5)',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
              <div>
                <span class="status-badge" style={{ fontSize: 'var(--font-size-xs)', marginRight: 'var(--space-2)' }}>
                  {labels.typeLabels[update.type] || update.type}
                </span>
                <span class="text-muted" style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--font-size-xs)' }}>
                  {formatDate(update.created_at)}
                </span>
              </div>
              {isOwner && (
                <button
                  class="btn btn-sm"
                  onClick={() => handleDelete(update.id)}
                  style={{ fontSize: 'var(--font-size-xs)', padding: '2px 8px', color: 'var(--color-text-muted)' }}
                  aria-label="Delete update"
                >
                  ×
                </button>
              )}
            </div>
            <h3 style={{ margin: 'var(--space-1) 0', fontSize: 'var(--font-size-md)' }}>{update.title}</h3>
            {update.content && (
              <p style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--font-size-sm)', marginTop: 'var(--space-1)' }}>
                {update.content}
              </p>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default BuildUpdatesIsland;
