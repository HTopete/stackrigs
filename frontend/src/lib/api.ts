// API base URL — set via PUBLIC_API_URL env var at build time.
// In production: https://api.stackrigs.com
// In dev: empty string (proxied by Astro dev server or same-origin)
export const API_BASE = import.meta.env.PUBLIC_API_URL || '';

// --- API response types (match Go models) ---

interface APIBuilder {
  id: number;
  handle: string;
  display_name: string;
  bio?: string;
  avatar_url?: string;
  website?: string;
  github_url?: string;
  twitter_url?: string;
  created_at: string;
  updated_at: string;
}

interface APITechnology {
  id: number;
  name: string;
  slug: string;
  category?: string;
  build_count?: number;
}

interface APIBuildUpdate {
  id: number;
  build_id: number;
  type: string;
  title: string;
  content: string;
  created_at: string;
}

interface APIBuild {
  id: number;
  builder_id: number;
  builder?: APIBuilder;
  name: string;
  description: string;
  status: string;
  repo_url?: string;
  live_url?: string;
  cover_image?: string;
  what_works?: string;
  what_broke?: string;
  what_id_change?: string;
  technologies?: APITechnology[];
  updates?: APIBuildUpdate[];
  created_at: string;
  updated_at: string;
}

interface APIPaginatedResponse<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
  has_more: boolean;
}

// --- Frontend types (what components expect) ---

export interface BuildCardData {
  id: string;
  name: string;
  tagline?: string;
  status: 'building' | 'launched' | 'paused' | 'abandoned';
  stack?: Array<{ slug: string; displayName: string }>;
  whatWorks?: string;
  lastUpdated: string;
  builderHandle?: string;
  coverImage?: string;
}

export interface BuildDetailData extends BuildCardData {
  builderName?: string;
  whatBroke?: string;
  whatIdChange?: string;
  updates?: Array<{
    type: 'pivot' | 'stack_change' | 'milestone' | 'reflection' | 'infra_change' | 'tool_change';
    date: string;
    title: string;
    body: string;
  }>;
}

export interface BuilderData {
  handle: string;
  name: string;
  bio: string;
  avatar: string;
  memberSince: string;
  lastActive: string;
  links: {
    github?: string;
    twitter?: string;
    website?: string;
  };
}

export interface TechData {
  slug: string;
  displayName: string;
  count: number;
  category?: string;
}

// --- Transform helpers ---

function toBuildCard(b: APIBuild): BuildCardData {
  return {
    id: String(b.id),
    name: b.name,
    tagline: b.description || undefined,
    status: b.status as BuildCardData['status'],
    stack: b.technologies?.map(t => ({ slug: t.slug, displayName: t.name })),
    whatWorks: b.what_works || undefined,
    lastUpdated: b.updated_at,
    builderHandle: b.builder?.handle,
    coverImage: b.cover_image || undefined,
  };
}

function toBuildDetail(b: APIBuild): BuildDetailData {
  return {
    ...toBuildCard(b),
    builderName: b.builder?.display_name,
    whatBroke: b.what_broke || undefined,
    whatIdChange: b.what_id_change || undefined,
    updates: b.updates?.map(u => ({
      type: (u.type || 'milestone') as BuildDetailData['updates'][0]['type'],
      date: u.created_at,
      title: u.title || '',
      body: u.content,
    })),
  };
}

function toBuilder(b: APIBuilder): BuilderData {
  return {
    handle: b.handle,
    name: b.display_name,
    bio: b.bio || '',
    avatar: b.avatar_url || '/avatars/placeholder.png',
    memberSince: b.created_at,
    lastActive: b.updated_at,
    links: {
      github: b.github_url || undefined,
      twitter: b.twitter_url || undefined,
      website: b.website || undefined,
    },
  };
}

// --- Fetch helper ---

async function apiFetch<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(`${API_BASE}${path}`);
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

// --- Public API functions ---

export async function getBuilds(params?: {
  tech?: string;
  status?: string;
  sort?: string;
  builder?: string;
  limit?: number;
  offset?: number;
}): Promise<{ builds: BuildCardData[]; total: number; hasMore: boolean }> {
  const searchParams = new URLSearchParams();
  if (params?.tech) searchParams.set('tech', params.tech);
  if (params?.status) searchParams.set('status', params.status);
  if (params?.sort) searchParams.set('sort', params.sort);
  if (params?.builder) searchParams.set('builder', params.builder);
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));

  const qs = searchParams.toString();
  const data = await apiFetch<APIPaginatedResponse<APIBuild>>(`/api/builds${qs ? `?${qs}` : ''}`);
  if (!data) return { builds: [], total: 0, hasMore: false };
  return {
    builds: data.data.map(toBuildCard),
    total: data.total,
    hasMore: data.has_more,
  };
}

export async function getBuild(id: string | number): Promise<BuildDetailData | null> {
  const data = await apiFetch<APIBuild>(`/api/builds/${id}`);
  if (!data) return null;
  return toBuildDetail(data);
}

export async function getBuilder(handle: string): Promise<BuilderData | null> {
  const data = await apiFetch<APIBuilder>(`/api/builders/${handle}`);
  if (!data) return null;
  return toBuilder(data);
}

export async function getTechnologies(): Promise<TechData[]> {
  const data = await apiFetch<APITechnology[]>('/api/technologies');
  if (!data) return [];
  return data.map(t => ({ slug: t.slug, displayName: t.name, count: t.build_count || 0, category: t.category || '' }));
}

export async function getAllBuilderHandles(): Promise<string[]> {
  // Fetch a large page of builds to extract unique handles for static paths
  const data = await apiFetch<APIPaginatedResponse<APIBuild>>('/api/builds?limit=100');
  if (!data) return [];
  const handles = new Set(data.data.map(b => b.builder?.handle).filter(Boolean) as string[]);
  return [...handles];
}

export async function getAllBuildIds(): Promise<string[]> {
  const data = await apiFetch<APIPaginatedResponse<APIBuild>>('/api/builds?limit=100');
  if (!data) return [];
  return data.data.map(b => String(b.id));
}
