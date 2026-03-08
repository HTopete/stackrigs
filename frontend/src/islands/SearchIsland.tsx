import { useState, useEffect, useRef, useCallback } from 'preact/hooks';
import type { FunctionComponent } from 'preact';
import { API_BASE } from '../lib/api';

interface SearchResultItem {
  type: 'builder' | 'build' | 'technology';
  id: string | number;
  name: string;
  subtitle?: string;
  url: string;
}

interface BackendSearchResult {
  builders: Array<{ id: number; handle: string; display_name: string; bio?: string }>;
  builds: Array<{ id: number; name: string; description?: string; status?: string }>;
  technologies: Array<{ id: number; name: string; slug: string; category?: string }>;
}

interface Props {
  locale: string;
  placeholder: string;
  noResultsText: string;
  loadingText: string;
  groupLabels?: {
    builders: string;
    builds: string;
    technologies: string;
  };
}

function mapBackendToItems(data: BackendSearchResult, locale: string): SearchResultItem[] {
  const prefix = locale === 'en' ? '' : `/${locale}`;
  const items: SearchResultItem[] = [];

  for (const b of data.builders || []) {
    items.push({
      type: 'builder',
      id: String(b.id),
      name: b.display_name,
      subtitle: `@${b.handle}`,
      url: `${prefix}/${b.handle}`,
    });
  }

  for (const b of data.builds || []) {
    items.push({
      type: 'build',
      id: String(b.id),
      name: b.name,
      subtitle: b.description,
      url: `${prefix}/build/${b.id}`,
    });
  }

  for (const t of data.technologies || []) {
    items.push({
      type: 'technology',
      id: String(t.id),
      name: t.name,
      subtitle: t.category,
      url: `${prefix}/stack/${t.slug}`,
    });
  }

  return items;
}

const SearchIsland: FunctionComponent<Props> = ({ locale, placeholder, noResultsText, loadingText, groupLabels: groupLabelsProp }) => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResultItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [isOpen, setIsOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  const performSearch = useCallback(async (searchQuery: string) => {
    const trimmed = searchQuery.trim();
    if (!trimmed) {
      setResults([]);
      setIsOpen(false);
      return;
    }

    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/search?q=${encodeURIComponent(trimmed)}`);
      if (res.ok) {
        const data: BackendSearchResult = await res.json();
        const items = mapBackendToItems(data, locale);
        setResults(items);
        setActiveIndex(-1);
        setIsOpen(true);
      } else {
        setResults([]);
        setIsOpen(true);
      }
    } catch {
      setResults([]);
      setIsOpen(true);
    } finally {
      setLoading(false);
    }
  }, [locale]);

  const handleInput = (e: Event) => {
    const value = (e.target as HTMLInputElement).value;
    setQuery(value);

    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => performSearch(value), 300);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (!isOpen || results.length === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex(prev => (prev + 1) % results.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex(prev => (prev <= 0 ? results.length - 1 : prev - 1));
    } else if (e.key === 'Enter' && activeIndex >= 0) {
      e.preventDefault();
      const selected = results[activeIndex];
      if (selected) {
        window.location.href = selected.url;
      }
    } else if (e.key === 'Escape') {
      setIsOpen(false);
      setActiveIndex(-1);
    }
  };

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  const grouped = {
    builder: results.filter(r => r.type === 'builder'),
    build: results.filter(r => r.type === 'build'),
    technology: results.filter(r => r.type === 'technology'),
  };

  const groupLabels: Record<string, string> = {
    builder: groupLabelsProp?.builders || 'Builders',
    build: groupLabelsProp?.builds || 'Builds',
    technology: groupLabelsProp?.technologies || 'Technologies',
  };

  let globalIndex = -1;

  return (
    <div class="search-island" role="search">
      <div class="search-input-wrapper">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          ref={inputRef}
          type="search"
          class="search-input"
          placeholder={placeholder}
          value={query}
          onInput={handleInput}
          onKeyDown={handleKeyDown}
          onFocus={() => query.trim() && results.length > 0 && setIsOpen(true)}
          onBlur={() => setTimeout(() => setIsOpen(false), 200)}
          aria-label="Search"
          aria-expanded={isOpen}
          aria-controls="search-results"
          aria-activedescendant={activeIndex >= 0 ? `search-result-${activeIndex}` : undefined}
          role="combobox"
          autocomplete="off"
        />
      </div>

      {loading && (
        <div class="search-loading" aria-live="polite">
          <span>{loadingText}...</span>
        </div>
      )}

      {isOpen && !loading && (
        <div class="search-results" id="search-results" role="listbox">
          {results.length === 0 && query.trim() && (
            <div class="search-no-results">{noResultsText}</div>
          )}

          {Object.entries(grouped).map(([type, items]) => {
            if (items.length === 0) return null;
            return (
              <div class="search-group" key={type}>
                <div class="search-group-label">{groupLabels[type]}</div>
                {items.map(item => {
                  globalIndex++;
                  const idx = globalIndex;
                  return (
                    <a
                      key={item.id}
                      id={`search-result-${idx}`}
                      href={item.url}
                      class={`search-result-item ${idx === activeIndex ? 'search-result-active' : ''}`}
                      role="option"
                      aria-selected={idx === activeIndex}
                    >
                      <span class="search-result-name">{item.name}</span>
                      {item.subtitle && <span class="search-result-subtitle">{item.subtitle}</span>}
                    </a>
                  );
                })}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};

export default SearchIsland;
