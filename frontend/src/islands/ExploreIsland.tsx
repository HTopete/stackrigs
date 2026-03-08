import { useState, useEffect, useCallback } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface BuildCardData {
  id: string;
  name: string;
  tagline?: string;
  status: string;
  stack?: Array<{ slug: string; displayName: string }>;
  whatWorks?: string;
  lastUpdated: string;
  builderHandle?: string;
}

interface TechOption {
  slug: string;
  displayName: string;
}

interface Props {
  initialBuilds: BuildCardData[];
  technologies: TechOption[];
  locale: string;
  labels: {
    technology: string;
    status: string;
    sortBy: string;
    sortUpdated: string;
    sortCreated: string;
    noBuilds: string;
    statusLabels: Record<string, string>;
  };
}

const statuses = ['building', 'launched', 'paused', 'abandoned'];

const ExploreIsland: FunctionComponent<Props> = ({ initialBuilds, technologies, locale, labels }) => {
  const [builds, setBuilds] = useState<BuildCardData[]>(initialBuilds);
  const [selectedTechs, setSelectedTechs] = useState<Set<string>>(new Set());
  const [selectedStatuses, setSelectedStatuses] = useState<Set<string>>(new Set());
  const [sort, setSort] = useState('updated');
  const [loading, setLoading] = useState(false);

  const prefix = locale === 'en' ? '' : `/${locale}`;

  const fetchBuilds = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (selectedTechs.size > 0) params.set('tech', [...selectedTechs].join(','));
      if (selectedStatuses.size > 0) params.set('status', [...selectedStatuses].join(','));
      params.set('sort', sort);
      params.set('limit', '20');

      const res = await fetch(`${API_BASE}/api/builds?${params}`);
      if (res.ok) {
        const data = await res.json();
        const items = (data.data || []).map((b: any) => ({
          id: String(b.id),
          name: b.name,
          tagline: b.description || b.tagline,
          status: b.status,
          stack: (b.technologies || []).map((t: any) => ({ slug: t.slug, displayName: t.name })),
          whatWorks: b.what_works,
          lastUpdated: b.updated_at,
          builderHandle: b.builder?.handle,
        }));
        setBuilds(items);
      }
    } catch {
      // Keep current builds on error
    } finally {
      setLoading(false);
    }
  }, [selectedTechs, selectedStatuses, sort]);

  useEffect(() => {
    fetchBuilds();
  }, [fetchBuilds]);

  const toggleTech = (slug: string) => {
    setSelectedTechs(prev => {
      const next = new Set(prev);
      if (next.has(slug)) next.delete(slug);
      else next.add(slug);
      return next;
    });
  };

  const toggleStatus = (status: string) => {
    setSelectedStatuses(prev => {
      const next = new Set(prev);
      if (next.has(status)) next.delete(status);
      else next.add(status);
      return next;
    });
  };

  return (
    <div class="explore-layout">
      <aside class="filters" aria-label="Filter by">
        <div class="filter-group">
          <h3 class="filter-group-title">{labels.technology}</h3>
          {technologies.map(tech => (
            <label class="filter-option" key={tech.slug}>
              <input
                type="checkbox"
                checked={selectedTechs.has(tech.slug)}
                onChange={() => toggleTech(tech.slug)}
              />
              {tech.displayName}
            </label>
          ))}
        </div>

        <div class="filter-group">
          <h3 class="filter-group-title">{labels.status}</h3>
          {statuses.map(status => (
            <label class="filter-option" key={status}>
              <input
                type="checkbox"
                checked={selectedStatuses.has(status)}
                onChange={() => toggleStatus(status)}
              />
              {labels.statusLabels[status] || status}
            </label>
          ))}
        </div>

        <div class="filter-group">
          <h3 class="filter-group-title">{labels.sortBy}</h3>
          <label class="filter-option">
            <input
              type="radio"
              name="sort"
              checked={sort === 'updated'}
              onChange={() => setSort('updated')}
            />
            {labels.sortUpdated}
          </label>
          <label class="filter-option">
            <input
              type="radio"
              name="sort"
              checked={sort === 'created'}
              onChange={() => setSort('created')}
            />
            {labels.sortCreated}
          </label>
        </div>
      </aside>

      <div>
        {loading && (
          <div style={{ textAlign: 'center', padding: 'var(--space-8)', color: 'var(--color-text-muted)' }}>
            Loading...
          </div>
        )}

        {!loading && builds.length === 0 && (
          <p class="text-muted" style={{ textAlign: 'center', padding: 'var(--space-8)' }}>
            {labels.noBuilds}
          </p>
        )}

        {!loading && builds.length > 0 && (
          <div class="grid-cards">
            {builds.map(build => (
              <article class="card build-card" key={build.id}>
                <div class="card-meta">
                  <span class={`status-badge status-${build.status}`}>
                    {labels.statusLabels[build.status] || build.status}
                  </span>
                </div>
                <h3 class="build-card-title">
                  <a href={`${prefix}/build/${build.id}`} class="build-card-link">
                    {build.name}
                  </a>
                </h3>
                {build.tagline && <p class="build-card-tagline">{build.tagline}</p>}
                {build.stack && build.stack.length > 0 && (
                  <div class="build-card-tags">
                    {build.stack.slice(0, 5).map(tech => (
                      <a href={`${prefix}/stack/${tech.slug}`} class="tag" key={tech.slug}>
                        {tech.displayName}
                      </a>
                    ))}
                    {build.stack.length > 5 && (
                      <span class="tag text-muted">+{build.stack.length - 5}</span>
                    )}
                  </div>
                )}
                {build.whatWorks && (
                  <div class="build-card-works">
                    <span class="build-card-works-label text-accent-green" aria-hidden="true">&#10003;</span>
                    <span class="line-clamp-3">{build.whatWorks}</span>
                  </div>
                )}
              </article>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default ExploreIsland;
