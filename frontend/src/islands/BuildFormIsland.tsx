import { useState, useEffect } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface BuildData {
  id?: number;
  name: string;
  description: string;
  status: string;
  repo_url: string;
  live_url: string;
  what_works: string;
  what_broke: string;
  what_id_change: string;
  technologies: string[];
}

interface Props {
  mode: 'create' | 'edit';
  buildId?: string;
  locale: string;
  labels: {
    createTitle: string;
    editTitle: string;
    name: string;
    namePlaceholder: string;
    description: string;
    descriptionPlaceholder: string;
    status: string;
    repoUrl: string;
    repoUrlPlaceholder: string;
    liveUrl: string;
    liveUrlPlaceholder: string;
    technologies: string;
    technologiesPlaceholder: string;
    technologiesHint: string;
    whatWorks: string;
    whatWorksPlaceholder: string;
    whatBroke: string;
    whatBrokePlaceholder: string;
    whatIdChange: string;
    whatIdChangePlaceholder: string;
    submit: string;
    submitEdit: string;
    saving: string;
    loginRequired: string;
    success: string;
    error: string;
  };
  statusLabels: {
    building: string;
    launched: string;
    paused: string;
    abandoned: string;
  };
}

const emptyForm: BuildData = {
  name: '',
  description: '',
  status: 'building',
  repo_url: '',
  live_url: '',
  what_works: '',
  what_broke: '',
  what_id_change: '',
  technologies: [],
};

const BuildFormIsland: FunctionComponent<Props> = ({ mode, buildId, locale, labels, statusLabels }) => {
  const [form, setForm] = useState<BuildData>(emptyForm);
  const [techInput, setTechInput] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);
  const [authed, setAuthed] = useState<boolean | null>(null);

  useEffect(() => {
    fetch(`${API_BASE}/api/auth/me`, { credentials: 'include' })
      .then(res => {
        setAuthed(res.ok);
        if (!res.ok) return;
        if (mode === 'edit' && buildId) {
          return fetch(`${API_BASE}/api/builds/${buildId}`, { credentials: 'include' })
            .then(r => r.ok ? r.json() : null)
            .then(data => {
              if (data) {
                setForm({
                  id: data.id,
                  name: data.name || '',
                  description: data.description || '',
                  status: data.status || 'building',
                  repo_url: data.repo_url || '',
                  live_url: data.live_url || '',
                  what_works: data.what_works || '',
                  what_broke: data.what_broke || '',
                  what_id_change: data.what_id_change || '',
                  technologies: (data.technologies || []).map((t: any) => t.slug),
                });
                setTechInput((data.technologies || []).map((t: any) => t.slug).join(', '));
              }
            });
        }
      })
      .catch(() => setAuthed(false));
  }, []);

  if (authed === null) return null;

  if (!authed) {
    const prefix = locale === 'en' ? '' : `/${locale}`;
    return (
      <div class="build-form-auth">
        <p>{labels.loginRequired}</p>
        <a href={`${prefix}/signin`} class="btn btn-primary">{labels.loginRequired}</a>
      </div>
    );
  }

  const updateField = (field: keyof BuildData, value: string) => {
    setForm(prev => ({ ...prev, [field]: value }));
  };

  const handleTechChange = (value: string) => {
    setTechInput(value);
    const techs = value.split(',').map(s => s.trim().toLowerCase()).filter(Boolean);
    setForm(prev => ({ ...prev, technologies: techs }));
  };

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setSaving(true);
    setError('');
    setSuccess(false);

    try {
      const url = mode === 'edit'
        ? `${API_BASE}/api/builds/${buildId}`
        : `${API_BASE}/api/builds`;

      const method = mode === 'edit' ? 'PUT' : 'POST';

      const body = mode === 'edit'
        ? {
            name: form.name || undefined,
            description: form.description || undefined,
            status: form.status || undefined,
            repo_url: form.repo_url || undefined,
            live_url: form.live_url || undefined,
            what_works: form.what_works || undefined,
            what_broke: form.what_broke || undefined,
            what_id_change: form.what_id_change || undefined,
            technologies: form.technologies.length > 0 ? form.technologies : undefined,
          }
        : {
            name: form.name,
            description: form.description,
            status: form.status,
            repo_url: form.repo_url,
            live_url: form.live_url,
            what_works: form.what_works,
            what_broke: form.what_broke,
            what_id_change: form.what_id_change,
            technologies: form.technologies,
          };

      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(body),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setError(data?.error || labels.error);
        return;
      }

      const created = await res.json();
      setSuccess(true);

      setTimeout(() => {
        const prefix = locale === 'en' ? '' : `/${locale}`;
        window.location.href = `${prefix}/build/${created.id}`;
      }, 800);
    } catch {
      setError(labels.error);
    } finally {
      setSaving(false);
    }
  };

  const title = mode === 'edit' ? labels.editTitle : labels.createTitle;
  const submitText = saving ? labels.saving : (mode === 'edit' ? labels.submitEdit : labels.submit);

  return (
    <form class="build-form" onSubmit={handleSubmit}>
      <h1 class="build-form-title">{title}</h1>

      {error && <div class="build-form-error" role="alert">{error}</div>}
      {success && <div class="build-form-success" role="status">{labels.success}</div>}

      <div class="build-form-field">
        <label for="bf-name">{labels.name} *</label>
        <input
          id="bf-name"
          type="text"
          value={form.name}
          onInput={(e) => updateField('name', (e.target as HTMLInputElement).value)}
          placeholder={labels.namePlaceholder}
          required
        />
      </div>

      <div class="build-form-field">
        <label for="bf-description">{labels.description}</label>
        <textarea
          id="bf-description"
          value={form.description}
          onInput={(e) => updateField('description', (e.target as HTMLTextAreaElement).value)}
          placeholder={labels.descriptionPlaceholder}
          rows={3}
        />
      </div>

      <div class="build-form-row">
        <div class="build-form-field">
          <label for="bf-status">{labels.status}</label>
          <select
            id="bf-status"
            value={form.status}
            onChange={(e) => updateField('status', (e.target as HTMLSelectElement).value)}
          >
            <option value="building">{statusLabels.building}</option>
            <option value="launched">{statusLabels.launched}</option>
            <option value="paused">{statusLabels.paused}</option>
            <option value="abandoned">{statusLabels.abandoned}</option>
          </select>
        </div>

        <div class="build-form-field">
          <label for="bf-tech">{labels.technologies}</label>
          <input
            id="bf-tech"
            type="text"
            value={techInput}
            onInput={(e) => handleTechChange((e.target as HTMLInputElement).value)}
            placeholder={labels.technologiesPlaceholder}
          />
          <span class="build-form-hint">{labels.technologiesHint}</span>
        </div>
      </div>

      <div class="build-form-row">
        <div class="build-form-field">
          <label for="bf-repo">{labels.repoUrl}</label>
          <input
            id="bf-repo"
            type="url"
            value={form.repo_url}
            onInput={(e) => updateField('repo_url', (e.target as HTMLInputElement).value)}
            placeholder={labels.repoUrlPlaceholder}
          />
        </div>

        <div class="build-form-field">
          <label for="bf-live">{labels.liveUrl}</label>
          <input
            id="bf-live"
            type="url"
            value={form.live_url}
            onInput={(e) => updateField('live_url', (e.target as HTMLInputElement).value)}
            placeholder={labels.liveUrlPlaceholder}
          />
        </div>
      </div>

      <div class="build-form-field">
        <label for="bf-works">{labels.whatWorks}</label>
        <textarea
          id="bf-works"
          value={form.what_works}
          onInput={(e) => updateField('what_works', (e.target as HTMLTextAreaElement).value)}
          placeholder={labels.whatWorksPlaceholder}
          rows={3}
        />
      </div>

      <div class="build-form-field">
        <label for="bf-broke">{labels.whatBroke}</label>
        <textarea
          id="bf-broke"
          value={form.what_broke}
          onInput={(e) => updateField('what_broke', (e.target as HTMLTextAreaElement).value)}
          placeholder={labels.whatBrokePlaceholder}
          rows={3}
        />
      </div>

      <div class="build-form-field">
        <label for="bf-change">{labels.whatIdChange}</label>
        <textarea
          id="bf-change"
          value={form.what_id_change}
          onInput={(e) => updateField('what_id_change', (e.target as HTMLTextAreaElement).value)}
          placeholder={labels.whatIdChangePlaceholder}
          rows={3}
        />
      </div>

      <button type="submit" class="btn btn-primary build-form-submit" disabled={saving}>
        {submitText}
      </button>
    </form>
  );
};

export default BuildFormIsland;
