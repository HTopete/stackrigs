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
  coverImage?: string;
}

interface TechOption {
  slug: string;
  displayName: string;
  count?: number;
  category?: string;
}

interface Props {
  initialBuilds: BuildCardData[];
  initialTotal: number;
  technologies: TechOption[];
  locale: string;
  labels: {
    technology: string;
    status: string;
    sortBy: string;
    sortUpdated: string;
    sortCreated: string;
    noBuilds: string;
    clearFilters: string;
    showMore: string;
    showLess: string;
    results: string;
    statusLabels: Record<string, string>;
  };
}

const statuses = ['building', 'launched', 'paused', 'abandoned'];
const TECH_VISIBLE_DEFAULT = 8;

const SkeletonCard = () => (
  <div class="card build-card skeleton-card" aria-hidden="true">
    <div class="skeleton skeleton-badge" />
    <div class="skeleton skeleton-title" />
    <div class="skeleton skeleton-line" />
    <div class="skeleton skeleton-line skeleton-line--short" />
    <div class="skeleton-tags">
      <div class="skeleton skeleton-tag" />
      <div class="skeleton skeleton-tag" />
      <div class="skeleton skeleton-tag" />
    </div>
  </div>
);

const ExploreIsland: FunctionComponent<Props> = ({
  initialBuilds,
  initialTotal,
  technologies,
  locale,
  labels,
}) => {
  const [builds, setBuilds] = useState<BuildCardData[]>(initialBuilds);
  const [total, setTotal] = useState(initialTotal);
  const [selectedTechs, setSelectedTechs] = useState<Set<string>>(new Set());
  const [selectedStatus, setSelectedStatus] = useState('');
  const [sort, setSort] = useState('updated');
  const [loading, setLoading] = useState(false);
  const [techExpanded, setTechExpanded] = useState(false);

  const prefix = locale === 'en' ? '' : `/${locale}`;
  const hasFilters = selectedTechs.size > 0 || selectedStatus !== '';

  // Group technologies by category, with uncategorised last
  const techByCategory = technologies.reduce<Record<string, TechOption[]>>((acc, t) => {
    const cat = t.category || 'other';
    if (!acc[cat]) acc[cat] = [];
    acc[cat].push(t);
    return acc;
  }, {});
  const categoryOrder = ['language', 'runtime', 'framework', 'database', 'infrastructure', 'tool', 'ai', 'other'];
  const sortedCategories = Object.keys(techByCategory).sort(
    (a, b) => (categoryOrder.indexOf(a) ?? 99) - (categoryOrder.indexOf(b) ?? 99)
  );

  const visibleTechs = techExpanded ? technologies : technologies.slice(0, TECH_VISIBLE_DEFAULT);
  const useGrouped = sortedCategories.length > 1;

  const fetchBuilds = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      for (const slug of selectedTechs) params.append('tech', slug);
      if (selectedStatus) params.set('status', selectedStatus);
      params.set('sort', sort);
      params.set('limit', '20');

      const res = await fetch(`${API_BASE}/api/builds?${params}`);
      if (res.ok) {
        const data = await res.json();
        setTotal(data.total ?? 0);
        setBuilds((data.data || []).map((b: any) => ({
          id: String(b.id),
          name: b.name,
          tagline: b.description || b.tagline,
          status: b.status,
          stack: (b.technologies || []).map((t: any) => ({ slug: t.slug, displayName: t.name })),
          whatWorks: b.what_works,
          lastUpdated: b.updated_at,
          builderHandle: b.builder?.handle,
          coverImage: b.cover_image || undefined,
        })));
      }
    } catch {
      // Keep current builds on network error
    } finally {
      setLoading(false);
    }
  }, [selectedTechs, selectedStatus, sort]);

  useEffect(() => {
    fetchBuilds();
  }, [fetchBuilds]);

  const toggleTech = (slug: string) => {
    setSelectedTechs(prev => {
      const next = new Set(prev);
      next.has(slug) ? next.delete(slug) : next.add(slug);
      return next;
    });
  };

  const clearFilters = () => {
    setSelectedTechs(new Set());
    setSelectedStatus('');
  };

  return (
    <div class="explore-layout">
      {/* Sidebar filters */}
      <aside class="filters" aria-label="Filter builds">

        {hasFilters && (
          <button class="clear-filters-btn" onClick={clearFilters}>
            ✕ {labels.clearFilters}
          </button>
        )}

        <div class="filter-group">
          <h3 class="filter-group-title">{labels.technology}</h3>
          {useGrouped
            ? sortedCategories.map(cat => (
                <div class="filter-category" key={cat}>
                  <span class="filter-category-label">{cat}</span>
                  {(techByCategory[cat] || []).map(tech => (
                    <label class={`filter-option${selectedTechs.has(tech.slug) ? ' filter-option--active' : ''}`} key={tech.slug}>
                      <input
                        type="checkbox"
                        checked={selectedTechs.has(tech.slug)}
                        onChange={() => toggleTech(tech.slug)}
                      />
                      <span class="filter-option-label">{tech.displayName}</span>
                      {tech.count != null && (
                        <span class="filter-option-count font-mono">{tech.count}</span>
                      )}
                    </label>
                  ))}
                </div>
              ))
            : (
              <>
                {visibleTechs.map(tech => (
                  <label class={`filter-option${selectedTechs.has(tech.slug) ? ' filter-option--active' : ''}`} key={tech.slug}>
                    <input
                      type="checkbox"
                      checked={selectedTechs.has(tech.slug)}
                      onChange={() => toggleTech(tech.slug)}
                    />
                    <span class="filter-option-label">{tech.displayName}</span>
                    {tech.count != null && (
                      <span class="filter-option-count font-mono">{tech.count}</span>
                    )}
                  </label>
                ))}
                {technologies.length > TECH_VISIBLE_DEFAULT && (
                  <button class="filter-show-more" onClick={() => setTechExpanded(v => !v)}>
                    {techExpanded ? labels.showLess : `${labels.showMore} (${technologies.length - TECH_VISIBLE_DEFAULT})`}
                  </button>
                )}
              </>
            )
          }
        </div>

        <div class="filter-group">
          <h3 class="filter-group-title">{labels.status}</h3>
          {statuses.map(status => (
            <label class={`filter-option${selectedStatus === status ? ' filter-option--active' : ''}`} key={status}>
              <input
                type="radio"
                name="status-filter"
                checked={selectedStatus === status}
                onChange={() => setSelectedStatus(selectedStatus === status ? '' : status)}
              />
              {labels.statusLabels[status] || status}
            </label>
          ))}
        </div>

        <div class="filter-group">
          <h3 class="filter-group-title">{labels.sortBy}</h3>
          <label class="filter-option">
            <input type="radio" name="sort" checked={sort === 'updated'} onChange={() => setSort('updated')} />
            {labels.sortUpdated}
          </label>
          <label class="filter-option">
            <input type="radio" name="sort" checked={sort === 'created'} onChange={() => setSort('created')} />
            {labels.sortCreated}
          </label>
        </div>
      </aside>

      {/* Results */}
      <div class="explore-results">
        {/* Result count */}
        <div class="explore-results-meta" aria-live="polite" aria-atomic="true">
          {!loading && (
            <span class="results-count font-mono text-muted">
              {total} {labels.results}
            </span>
          )}
          {hasFilters && !loading && (
            <button class="clear-filters-inline" onClick={clearFilters}>
              {labels.clearFilters}
            </button>
          )}
        </div>

        {/* Skeleton loading */}
        {loading && (
          <div class="grid-cards" aria-label="Loading builds">
            {Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}
          </div>
        )}

        {/* Empty state */}
        {!loading && builds.length === 0 && (
          <div class="explore-empty">
            <p class="text-muted">{labels.noBuilds}</p>
            {hasFilters && (
              <button class="btn btn-secondary" style="margin-top: var(--space-4);" onClick={clearFilters}>
                {labels.clearFilters}
              </button>
            )}
          </div>
        )}

        {/* Build cards */}
        {!loading && builds.length > 0 && (
          <div class="grid-cards">
            {builds.map(build => (
              <article class={`card build-card${build.coverImage ? ' has-cover' : ''}`} key={build.id}>
                {build.coverImage && (
                  <div class="build-card-cover">
                    <img src={build.coverImage} alt="" loading="lazy" decoding="async" />
                  </div>
                )}
                <div class="card-meta">
                  <span class={`badge badge-${build.status}`}>
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
                  <div class="build-card-tags" style="position:relative;z-index:2;">
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
                    <span aria-hidden="true">✓</span>
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
