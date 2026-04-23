import { useState, useEffect, useRef } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface AuthBuilder {
  id: number;
  handle: string;
  display_name: string;
  avatar_url: string;
}

interface Props {
  locale: string;
  signInText: string;
  signOutText: string;
  myProfileText: string;
  newBuildText: string;
  myBuildsText: string;
  editProfileText: string;
  signInHref: string;
}

const AuthIsland: FunctionComponent<Props> = ({ locale, signInText, signOutText, myProfileText, newBuildText, myBuildsText, editProfileText, signInHref }) => {
  const [builder, setBuilder] = useState<AuthBuilder | null>(null);
  const [checked, setChecked] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    fetch(`${API_BASE}/api/auth/me`, { credentials: 'include' })
      .then(res => {
        if (res.ok) return res.json();
        return null;
      })
      .then(data => {
        if (data?.builder) setBuilder(data.builder);
      })
      .catch(() => {})
      .finally(() => setChecked(true));
  }, []);

  useEffect(() => {
    if (!menuOpen) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener('click', handleClick);
    return () => document.removeEventListener('click', handleClick);
  }, [menuOpen]);

  const handleLogout = async () => {
    try {
      await fetch(`${API_BASE}/api/auth/logout`, {
        method: 'POST',
        credentials: 'include',
      });
    } catch {}
    setBuilder(null);
    setMenuOpen(false);
  };

  if (!checked) return null;

  if (!builder) {
    return (
      <a href={signInHref} class="btn btn-primary btn-sm">
        {signInText}
      </a>
    );
  }

  const prefix = locale === 'en' ? '' : `/${locale}`;
  const profileHref = `${prefix}/${builder.handle}`;

  return (
    <div class="auth-menu" ref={menuRef} style={{ position: 'relative' }}>
      <button
        class="auth-trigger"
        onClick={() => setMenuOpen(!menuOpen)}
        aria-expanded={menuOpen}
        aria-haspopup="true"
      >
        <img
          src={builder.avatar_url}
          alt={builder.display_name}
          width="28"
          height="28"
          style={{
            borderRadius: '50%',
            objectFit: 'cover',
          }}
        />
      </button>

      {menuOpen && (
        <div class="auth-dropdown" role="menu">
          <div class="auth-dropdown-header">
            <span class="auth-dropdown-name">{builder.display_name}</span>
            <span class="auth-dropdown-handle">@{builder.handle}</span>
          </div>
          <a href={`${prefix}/new-build`} class="auth-dropdown-item" role="menuitem">
            {newBuildText}
          </a>
          <a href={`${prefix}/my-builds`} class="auth-dropdown-item" role="menuitem">
            {myBuildsText}
          </a>
          <a href={profileHref} class="auth-dropdown-item" role="menuitem">
            {myProfileText}
          </a>
          <a href={`${prefix}/settings`} class="auth-dropdown-item" role="menuitem">
            {editProfileText}
          </a>
          <button
            class="auth-dropdown-item auth-dropdown-logout"
            onClick={handleLogout}
            role="menuitem"
          >
            {signOutText}
          </button>
        </div>
      )}
    </div>
  );
};

export default AuthIsland;
