import { useState, useEffect } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface ProfileData {
  display_name: string;
  bio: string;
  website: string;
  twitter_url: string;
  avatar_url: string;
}

interface Props {
  locale: string;
  labels: {
    title: string;
    displayName: string;
    bio: string;
    website: string;
    twitter: string;
    avatar: string;
    save: string;
    saving: string;
    saved: string;
    error: string;
    loginRequired: string;
    changeAvatar: string;
  };
}

const ProfileEditIsland: FunctionComponent<Props> = ({ locale, labels }) => {
  const [form, setForm] = useState<ProfileData>({
    display_name: '',
    bio: '',
    website: '',
    twitter_url: '',
    avatar_url: '',
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [avatarUploading, setAvatarUploading] = useState(false);

  useEffect(() => {
    fetch(`${API_BASE}/api/auth/me`, { credentials: 'include' })
      .then(res => {
        if (!res.ok) { setAuthed(false); return; }
        return res.json();
      })
      .then(data => {
        if (!data?.builder) { setAuthed(false); return; }
        setAuthed(true);
        const b = data.builder;
        setForm({
          display_name: b.display_name || '',
          bio: b.bio || '',
          website: b.website || '',
          twitter_url: b.twitter_url || '',
          avatar_url: b.avatar_url || '',
        });
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

  const updateField = (field: keyof ProfileData, value: string) => {
    setForm(prev => ({ ...prev, [field]: value }));
  };

  /** Resize + encode to WebP client-side, then upload the optimised blob. */
  const handleAvatarUpload = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    if (!input.files?.length) return;

    const file = input.files[0];
    if (file.size > 5 * 1024 * 1024) {
      setError('Avatar must be under 5 MB');
      return;
    }

    setAvatarUploading(true);
    setError('');

    try {
      const blob = await resizeToWebP(file, 256, 0.85);

      const formData = new FormData();
      formData.append('avatar', blob, 'avatar.webp');

      const res = await fetch(`${API_BASE}/api/upload/avatar`, {
        method: 'POST',
        credentials: 'include',
        body: formData,
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setError(data?.error || labels.error);
        return;
      }

      const data = await res.json();
      setForm(prev => ({ ...prev, avatar_url: data.url }));
    } catch {
      setError(labels.error);
    } finally {
      setAvatarUploading(false);
    }
  };

  /** Load an image file, resize to fit within `max` px, and return a WebP blob. */
  const resizeToWebP = (file: File, max: number, quality: number): Promise<Blob> =>
    new Promise((resolve, reject) => {
      const img = new Image();
      img.onload = () => {
        let { width: w, height: h } = img;
        if (w > max || h > max) {
          const ratio = Math.min(max / w, max / h);
          w = Math.round(w * ratio);
          h = Math.round(h * ratio);
        }
        const canvas = document.createElement('canvas');
        canvas.width = w;
        canvas.height = h;
        const ctx = canvas.getContext('2d')!;
        ctx.imageSmoothingQuality = 'high';
        ctx.drawImage(img, 0, 0, w, h);
        canvas.toBlob(
          (blob) => (blob ? resolve(blob) : reject(new Error('toBlob failed'))),
          'image/webp',
          quality,
        );
        URL.revokeObjectURL(img.src);
      };
      img.onerror = () => reject(new Error('image load failed'));
      img.src = URL.createObjectURL(file);
    });

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setSaving(true);
    setError('');
    setSuccess(false);

    try {
      const res = await fetch(`${API_BASE}/api/builders/me`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          display_name: form.display_name,
          bio: form.bio,
          website: form.website,
          twitter_url: form.twitter_url,
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setError(data?.error || labels.error);
        return;
      }

      setSuccess(true);
      setTimeout(() => setSuccess(false), 3000);
    } catch {
      setError(labels.error);
    } finally {
      setSaving(false);
    }
  };

  const submitText = saving ? labels.saving : labels.save;

  return (
    <form class="build-form" onSubmit={handleSubmit}>
      <h1 class="build-form-title">{labels.title}</h1>

      {error && <div class="build-form-error" role="alert">{error}</div>}
      {success && <div class="build-form-success" role="status">{labels.saved}</div>}

      {/* Avatar */}
      <div class="build-form-field">
        <label>{labels.avatar}</label>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-4)' }}>
          {form.avatar_url && (
            <img
              src={form.avatar_url}
              alt="Avatar"
              width="64"
              height="64"
              style={{ borderRadius: '50%', objectFit: 'cover' }}
            />
          )}
          <label class="btn btn-sm" style={{ cursor: 'pointer', opacity: avatarUploading ? 0.6 : 1 }}>
            {avatarUploading ? labels.saving : labels.changeAvatar}
            <input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              onChange={handleAvatarUpload}
              style={{ display: 'none' }}
              disabled={avatarUploading}
            />
          </label>
        </div>
      </div>

      <div class="build-form-field">
        <label for="pe-name">{labels.displayName}</label>
        <input
          id="pe-name"
          type="text"
          value={form.display_name}
          onInput={(e) => updateField('display_name', (e.target as HTMLInputElement).value)}
        />
      </div>

      <div class="build-form-field">
        <label for="pe-bio">{labels.bio}</label>
        <textarea
          id="pe-bio"
          value={form.bio}
          onInput={(e) => updateField('bio', (e.target as HTMLTextAreaElement).value)}
          rows={3}
        />
      </div>

      <div class="build-form-row">
        <div class="build-form-field">
          <label for="pe-website">{labels.website}</label>
          <input
            id="pe-website"
            type="url"
            value={form.website}
            onInput={(e) => updateField('website', (e.target as HTMLInputElement).value)}
            placeholder="https://..."
          />
        </div>

        <div class="build-form-field">
          <label for="pe-twitter">{labels.twitter}</label>
          <input
            id="pe-twitter"
            type="url"
            value={form.twitter_url}
            onInput={(e) => updateField('twitter_url', (e.target as HTMLInputElement).value)}
            placeholder="https://x.com/..."
          />
        </div>
      </div>

      <button type="submit" class="btn btn-primary build-form-submit" disabled={saving}>
        {submitText}
      </button>
    </form>
  );
};

export default ProfileEditIsland;
