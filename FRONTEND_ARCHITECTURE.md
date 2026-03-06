# StackRigs Frontend Architecture
## Astro 5.x + Editorial Zen Design System

**Fecha:** marzo 2026
**Autor:** Frontend Architecture Agent
**Estado:** Documento de diseno (pre-implementacion)

---

## 1. INVESTIGACION: Estado del Arte (marzo 2026)

### 1.1 Version de Astro

**Astro 5.18.0** es la ultima version estable (publicada ~28 feb 2026). Astro 6 esta en beta con dev server basado en workerd (Cloudflare), pero NO es estable. La decision correcta es usar **Astro 5.x** para produccion.

**Decision: Astro ^5.18.0** -- estable, maduro, con todas las features que necesitamos ya graduadas de experimental.

### 1.2 Features Nuevas de Astro 5 vs Astro 4

| Feature | Estado en 5.x | Relevancia para StackRigs |
|---------|---------------|--------------------------|
| Content Layer API | Estable | ALTA -- reemplaza Content Collections legacy |
| Server Islands | Estable | MEDIA -- util para InfraMetrics en tiempo real |
| Astro Actions | Estable (desde 4.15, maduro en 5.x) | BAJA -- no tenemos auth/forms complejos |
| CSP experimental | Experimental (estable en 6) | BAJA -- no critico para MVP |
| Live Content Collections | Experimental en 5.10 | NO USAR -- esperar a Astro 6 |

### 1.3 View Transitions API -- Soporte en Browsers

- **Same-document (SPA-style):** Chrome 111+, Edge 111+, Safari 18+, Firefox 133+ -- **soporte universal en 2026**
- **Cross-document (MPA):** Chrome 126+, Edge 126+ -- Chromium only, Safari/Firefox en progreso

**Decision:** Usar `<ViewTransitions />` de Astro para navegacion MPA con fallback graceful. En Chromium se obtienen transiciones cross-document nativas; en Safari/Firefox el contenido simplemente carga sin animacion. Zero-cost progressive enhancement.

### 1.4 Server Islands

Estables en Astro 5. Permiten renderizar componentes en el servidor de forma diferida, inyectandolos despues del HTML estatico inicial. Ideales para contenido que cambia frecuentemente sin sacrificar cache de la pagina completa.

**Decision:** Usar Server Islands para el componente `InfraMetrics` que muestra datos en tiempo real del Pi. El resto de la pagina se cachea como estatico.

### 1.5 Content Layer API vs Content Collections

Content Layer API (Astro 5) **reemplaza** las Content Collections legacy. Diferencias clave:

- Content Collections (Astro 4): Solo archivos locales (Markdown/MDX)
- Content Layer API (Astro 5): Cualquier fuente de datos -- APIs, bases de datos, CMS -- con type safety completo via loaders

**Decision:** Usar Content Layer con custom loaders que consumen nuestra Go API (`/api/builds`, `/api/technologies`, etc.) en build time. Esto genera paginas estaticas con datos frescos en cada deploy/rebuild.

### 1.6 Astro Actions

Estables desde Astro 4.15, maduros en 5.x. Proveen funciones server-side con validacion de input y type safety automatica. En 5.18 se anadio `security.actionBodySizeLimit` para controlar tamano de payload.

**Decision:** NO usar Actions para el MVP. StackRigs es primariamente read-only (el API de Go maneja los writes). Si en el futuro anadimos formularios de creacion de builds desde el frontend, Actions seria el camino natural.

### 1.7 i18n Sin Prefijos de URL

Astro 5 tiene i18n routing nativo. Configuracion:

```javascript
// astro.config.mjs
i18n: {
  locales: ['en', 'es'],
  defaultLocale: 'en',
  prefixDefaultLocale: false,  // /about (no /en/about)
  redirectToDefaultLocale: false
}
```

- Ingles: `/explore`, `/about`, `/build/123`
- Espanol: `/es/explore`, `/es/about`, `/es/build/123`

**Decision:** Ingles sin prefijo como locale default. Espanol con prefijo `/es/`. Traducciones en JSON plano (`i18n/en.json`, `i18n/es.json`) con helper function `t()` que resuelve por `Astro.currentLocale`.

---

## 2. ARQUITECTURA FRONTEND

### 2.1 Stack Tecnologico

| Capa | Tecnologia | Justificacion |
|------|-----------|---------------|
| Framework | Astro 5.18 | Content-driven, zero-JS default, islands |
| Islands | Preact | 3KB runtime vs 45KB React; JSX familiar; signals nativos |
| Styling | CSS vanilla + @layer | Sin runtime CSS, container queries nativas, light-dark() |
| Fonts | DM Serif Display + DM Sans + DM Mono | Editorial Zen design system |
| Search client | Fuse.js | <7KB, fuzzy search sin servidor |
| SSE client | EventSource nativo | Zero deps para InfraMetrics en tiempo real |
| i18n | Custom (JSON + t() helper) | Mas ligero que astro-i18n-aut; control total |
| Build | Vite (via Astro) | Ya incluido, zero config |

#### Por que Preact y no Svelte para islands:

Svelte produce bundles ligeramente mas pequenos por componente, pero Preact gana en este caso por:

1. **Ecosystem de senales (signals):** `@preact/signals` es 1.3KB y provee reactividad granular sin re-renders -- perfecto para el live metrics dashboard
2. **JSX directo:** Mismo mental model que el ecosistema React, reduce friccion para contribuidores
3. **Runtime constante:** 3KB de Preact se paga una vez; con solo 2-3 islands, la diferencia con Svelte es negligible
4. **Astro first-party:** `@astrojs/preact` es mantenido por el core team

### 2.2 Estructura de Directorios

```
frontend/
  astro.config.mjs           # Config: output hybrid, i18n, integrations
  package.json
  tsconfig.json               # Strict, paths aliases
  public/
    fonts/                     # Self-hosted DM fonts (WOFF2)
      dm-serif-display-*.woff2
      dm-sans-*.woff2
      dm-mono-*.woff2
    og/                        # OG image templates (SVG base)
    favicon.svg
    robots.txt
  src/
    content/
      config.ts                # Content Layer loaders definition
    layouts/
      Base.astro               # HTML shell: <html>, fonts, meta, OG, ViewTransitions
      Page.astro               # Base + Nav + Footer + slot
    pages/
      index.astro              # Landing: hero + builds recientes + stacks populares
      explore.astro            # Grid de builds con filtros (tech, status, sort)
      search.astro             # Pagina de busqueda con SearchIsland
      infra.astro              # Metricas real-time del Raspberry Pi
      about.astro              # Manifiesto + reglas del proyecto
      [handle].astro           # Perfil de builder (SSR o prerendered)
      build/
        [id].astro             # Build log detail con technologies, updates
      stack/
        [slug].astro           # Pagina por tecnologia -- SEO gold
      es/                      # Mirror para locale espanol
        index.astro
        explore.astro
        search.astro
        infra.astro
        about.astro
        [handle].astro
        build/
          [id].astro
        stack/
          [slug].astro
      og/
        [id].png.ts            # Endpoint para generar OG images dinamicas
    components/
      Nav.astro                # Navegacion principal + locale toggle
      Footer.astro             # Links, atribucion, status indicator
      BuildCard.astro          # Card de build: cover, nombre, builder, stacks, status
      BuildLog.astro           # Timeline de updates de un build
      StackTag.astro           # Pill/tag de tecnologia con link a /stack/[slug]
      StatusBadge.astro        # Badge de status: active/paused/shipped/archived
      FreshnessDot.astro       # Indicador visual de ultima actualizacion
      SearchBar.astro          # Shell estatica del search (hydrated por island)
      InfraMetrics.astro       # Server Island: datos del Pi con SSE fallback
      LocaleToggle.astro       # Switch en/es
      EmbedWidget.astro        # Preview del widget embebible (<iframe> snippet)
      SetupGallery.astro       # Galeria de fotos del setup (si aplica)
      Pagination.astro         # Paginacion para listas
      SkipLink.astro           # Accessibility: skip to content
      ThemeToggle.astro        # Light/dark mode switch
      Breadcrumb.astro         # Breadcrumbs para SEO + UX
    islands/                   # Componentes interactivos (Preact)
      SearchIsland.tsx         # Fuzzy search con Fuse.js, resultados live
      InfraLive.tsx            # Dashboard SSE: mem, uptime, req/min con signals
    styles/
      tokens.css               # Design tokens: colores, spacing, typography
      global.css               # Reset, @layer base, @font-face, fluid type
      layouts.css              # Layout primitives con container queries
    i18n/
      en.json                  # Traducciones ingles
      es.json                  # Traducciones espanol
      index.ts                 # Helper t(), getLocale(), getLocalizedPath()
    lib/
      api.ts                   # Client para la Go API (fetch wrapper con types)
      og.ts                    # Generador de OG images (satori + sharp)
      loaders.ts               # Content Layer loaders: builds, builders, technologies
      constants.ts             # URLs, limites, config compartida
    types/
      api.ts                   # TypeScript types mirroring Go models
```

### 2.3 Modos de Rendering por Pagina

| Pagina | Modo | Justificacion |
|--------|------|---------------|
| `index.astro` | Static (prerender) | Contenido cambia en deploy, no necesita SSR |
| `explore.astro` | Static con query params client-side | Filtros se aplican en JS con datos precargados |
| `search.astro` | Static | Island de Preact maneja la busqueda client-side |
| `infra.astro` | Hybrid (Server Island) | Metricas frescas via Server Island + SSE |
| `about.astro` | Static | Contenido fijo |
| `[handle].astro` | Static (prerendered) | Un HTML por builder, regenerado en rebuild |
| `build/[id].astro` | Static (prerendered) | Un HTML por build, regenerado en rebuild |
| `stack/[slug].astro` | Static (prerendered) | Un HTML por tech, SEO optimizado |
| `og/[id].png.ts` | SSR endpoint | Genera imagenes OG on-demand |

**Output mode: `hybrid`** -- paginas estaticas por default con opt-in a SSR donde se necesita.

### 2.4 Data Flow

```
                    BUILD TIME                          RUNTIME
                    ----------                          -------

  Go API (/api/*)                                  Go API (/api/*)
       |                                                |
       v                                                v
  Content Layer Loaders                            SSE / fetch
  (src/lib/loaders.ts)                             (islands/*.tsx)
       |                                                |
       v                                                v
  Astro Pages (.astro)                             Preact Islands
  [Static HTML generation]                         [Client interactivity]
       |                                                |
       v                                                v
  dist/ (HTML + minimal JS)  ----deploy--->  CDN / Static hosting
```

**Build time:** Los Content Layer loaders hacen fetch a la Go API, deserializan los datos con type safety, y Astro genera HTML estatico para cada pagina.

**Runtime:** Solo 2 islands necesitan JavaScript:
1. `SearchIsland.tsx` -- carga un indice JSON prebuild y usa Fuse.js para fuzzy search
2. `InfraLive.tsx` -- conecta via SSE a `/api/infra/stream` (o polling a `/api/infra`) para metricas live

### 2.5 Content Layer Loaders

```typescript
// Conceptual -- src/lib/loaders.ts
// Cada loader hace fetch a la Go API y retorna datos tipados

buildsLoader:    GET /api/builds?limit=1000     -> Build[]
buildersLoader:  GET /api/builders               -> Builder[]  (si se anade endpoint)
techLoader:      GET /api/technologies           -> Technology[]
```

Los loaders se registran en `src/content/config.ts` y producen colecciones queryables con `getCollection()` y `getEntry()`.

---

## 3. DESIGN SYSTEM: Editorial Zen

### 3.1 Design Tokens (tokens.css)

```
PALETA:
  --surface-0:    #FDFBF7     (warm off-white, fondo principal)
  --surface-1:    #F5F0E8     (cards, elementos elevados)
  --surface-2:    #EBE4D6     (hover, borders sutiles)
  --ink-primary:  #1A1A1A     (texto principal)
  --ink-secondary:#5C5C5C     (texto secundario)
  --ink-muted:    #8A8A8A     (placeholders, metadatos)
  --accent:       #C45D3E     (terracotta -- CTAs, links activos)
  --accent-hover: #A84E34     (hover del accent)
  --success:      #4A7C59     (status: shipped/active)
  --warning:      #C49A3E     (status: paused)
  --neutral:      #7A7A7A     (status: archived)

TIPOGRAFIA:
  --font-display: 'DM Serif Display', Georgia, serif
  --font-body:    'DM Sans', system-ui, sans-serif
  --font-mono:    'DM Mono', 'Cascadia Code', monospace

  --step--2:  clamp(0.72rem, 0.65vi + 0.55rem, 0.83rem)
  --step--1:  clamp(0.83rem, 0.78vi + 0.64rem, 1.00rem)
  --step-0:   clamp(1.00rem, 0.95vi + 0.76rem, 1.20rem)    /* base */
  --step-1:   clamp(1.20rem, 1.17vi + 0.91rem, 1.44rem)
  --step-2:   clamp(1.44rem, 1.45vi + 1.08rem, 1.73rem)
  --step-3:   clamp(1.73rem, 1.80vi + 1.28rem, 2.07rem)
  --step-4:   clamp(2.07rem, 2.24vi + 1.52rem, 2.49rem)
  --step-5:   clamp(2.49rem, 2.79vi + 1.80rem, 2.99rem)

SPACING (fluid):
  --space-3xs: clamp(0.25rem, 0.23vi + 0.19rem, 0.31rem)
  --space-2xs: clamp(0.50rem, 0.48vi + 0.38rem, 0.63rem)
  --space-xs:  clamp(0.75rem, 0.71vi + 0.57rem, 0.94rem)
  --space-s:   clamp(1.00rem, 0.95vi + 0.76rem, 1.25rem)
  --space-m:   clamp(1.50rem, 1.43vi + 1.14rem, 1.88rem)
  --space-l:   clamp(2.00rem, 1.90vi + 1.52rem, 2.50rem)
  --space-xl:  clamp(3.00rem, 2.86vi + 2.29rem, 3.75rem)
  --space-2xl: clamp(4.00rem, 3.81vi + 3.05rem, 5.00rem)
```

### 3.2 Arquitectura CSS

```css
/* @layer ordering in global.css */
@layer reset, tokens, base, layout, components, utilities;
```

**Principios:**

1. **Sin breakpoints, solo container queries.** Cada componente decide su layout basado en su contenedor, no en el viewport. Esto permite que `BuildCard` funcione identicamente en un grid de 3 columnas, en un sidebar, o standalone.

2. **`light-dark()` para temas.** Un solo set de custom properties que se adapta automaticamente:
   ```css
   :root { color-scheme: light dark; }
   --surface-0: light-dark(#FDFBF7, #1A1A1A);
   ```

3. **`:has()` para estados contextuales.** En lugar de clases toggled por JavaScript:
   ```css
   .build-card:has(.status-badge[data-status="shipped"]) { ... }
   ```

4. **Fluid typography sin media queries.** Los `clamp()` de arriba escalan suavemente de mobile a desktop. Se calculan con una escala mayor 1.2 (minor third).

5. **Zero runtime CSS.** No Tailwind, no CSS-in-JS, no PostCSS transforms. CSS vanilla con custom properties es suficiente en 2026.

### 3.3 Font Loading Strategy

```html
<!-- En Base.astro <head> -->
<link rel="preload" href="/fonts/dm-sans-variable.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="/fonts/dm-serif-display-regular.woff2" as="font" type="font/woff2" crossorigin>
```

- **Self-hosted** (no Google Fonts CDN) -- elimina conexion a terceros, mejor privacidad, sin CORS issues
- **Preload** solo los 2 weights criticos (body regular, display regular)
- **DM Mono** se carga lazy (solo aparece en code snippets)
- **font-display: swap** para evitar FOIT

---

## 4. PERFORMANCE Y WEB VITALS

### 4.1 Estrategia de Prefetch

```html
<!-- Speculation Rules en Base.astro -->
<script type="speculationrules">
{
  "prefetch": [{
    "source": "document",
    "where": { "selector_matches": "a[href^='/']" },
    "eagerness": "moderate"
  }],
  "prerender": [{
    "source": "document",
    "where": { "selector_matches": "a[data-prerender]" },
    "eagerness": "moderate"
  }]
}
</script>
```

**Justificacion:** Speculation Rules API funciona en Chromium (Chrome + Edge = ~70% de usuarios). Para Safari/Firefox, Astro tiene su propio prefetch con `<ViewTransitions />` que usa `<link rel="prefetch">` como fallback. Resultado: prefetch inteligente en todos los browsers.

- `prefetch` para todos los links internos con eagerness `moderate` (on hover/pointer proximity)
- `prerender` solo para links marcados con `data-prerender` (links de alta probabilidad: "Ver build", "Explorar")

### 4.2 Core Web Vitals Targets

| Metrica | Target | Estrategia |
|---------|--------|------------|
| LCP | < 1.0s | HTML estatico + fonts preloaded + zero render-blocking JS |
| INP | < 100ms | Solo 2 islands con Preact signals (no re-renders innecesarios) |
| CLS | 0 | Dimensiones explicitas en imagenes, font-display:swap con size-adjust |

### 4.3 JavaScript Budget

| Componente | Tamano estimado (gzip) |
|-----------|----------------------|
| Preact runtime | ~3 KB |
| @preact/signals | ~1.3 KB |
| SearchIsland + Fuse.js | ~8 KB |
| InfraLive | ~2 KB |
| **Total JS en pagina mas pesada** | **~14 KB** |
| Paginas sin islands | **0 KB** |

Para comparacion: la media de JS en sitios web en 2026 es ~500 KB. StackRigs envia 97% menos.

---

## 5. SEO Y OPEN GRAPH

### 5.1 Paginas SEO Gold

Las paginas `stack/[slug].astro` son el activo SEO principal. Cada tecnologia (React, Go, PostgreSQL, etc.) tiene su propia pagina con:

- `<title>` unico: "React Builds | StackRigs - Developer Build Index"
- `<meta name="description">` dinamico con count de builds
- Schema.org `CollectionPage` markup
- Links internos a cada build que usa esa tecnologia

### 5.2 Open Graph Dinamico

Endpoint `pages/og/[id].png.ts` genera imagenes OG usando `satori` (SVG -> PNG):

- **Build page:** Nombre del build + builder + stack tags + status
- **Builder page:** Avatar + handle + numero de builds
- **Stack page:** Nombre de la tech + numero de builds + top builders

Las imagenes se cachean con headers `Cache-Control: public, max-age=86400` (24h).

### 5.3 Structured Data

```json
// En build/[id].astro
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "Build name",
  "author": { "@type": "Person", "name": "Builder" },
  "applicationCategory": "DeveloperApplication"
}
```

---

## 6. ACCESSIBILITY

### 6.1 Principios

1. **Semantic HTML first.** `<nav>`, `<main>`, `<article>`, `<aside>`, `<section>` con headings jerarquicos. ARIA solo cuando HTML semantico no alcanza.
2. **Skip link** (`SkipLink.astro`) como primer elemento focusable
3. **Color contrast** minimo 4.5:1 (AA) para texto, 3:1 para UI components
4. **Focus visible** con outline personalizado que respeta el design system
5. **Reduced motion:** `@media (prefers-reduced-motion: reduce)` desactiva View Transitions y animaciones
6. **Keyboard navigation** completa en todos los islands

### 6.2 ARIA Patterns

- `SearchIsland`: `role="combobox"` + `aria-expanded` + `aria-activedescendant`
- `StatusBadge`: `aria-label` descriptivo ("Build status: active, last updated 2 days ago")
- `InfraLive`: `aria-live="polite"` para anunciar cambios de metricas

---

## 7. i18n ARCHITECTURE

### 7.1 Routing

```
/                    -> ingles (default, sin prefijo)
/explore             -> ingles
/build/42            -> ingles
/es/                 -> espanol
/es/explore          -> espanol
/es/build/42         -> espanol
```

### 7.2 Translation Helper

```typescript
// src/i18n/index.ts (conceptual)
export function t(key: string, locale?: string): string
export function getLocalizedPath(path: string, locale: string): string
export function getAlternateLinks(path: string): { lang: string, href: string }[]
```

### 7.3 SEO i18n

Cada pagina emite `<link rel="alternate" hreflang="en" href="...">` y `<link rel="alternate" hreflang="es" href="...">` para que Google indexe ambas versiones.

---

## 8. HOSTING Y DEPLOYMENT

### 8.1 Opciones Evaluadas

| Opcion | Veredicto | Razon |
|--------|-----------|-------|
| Cloudflare Pages | RECOMENDADO | Astro adapter oficial, edge SSR para Server Islands, free tier generoso |
| Vercel | Alternativa | Buen soporte Astro pero mas caro para SSR |
| Self-hosted (Pi) | NO | El Pi corre la Go API; separar concerns |
| Netlify | Alternativa | Funciona bien pero Cloudflare tiene edge mas rapido |

**Decision: Cloudflare Pages** con `@astrojs/cloudflare` adapter. Las paginas estaticas van al CDN global. El Server Island de InfraMetrics y el endpoint de OG images corren como Cloudflare Workers en el edge. Especialmente relevante dado que Astro 6 tendra soporte first-class para Cloudflare (indicador de la direccion del proyecto).

### 8.2 Build Trigger

El frontend se rebuilds cuando:
1. Push al repo del frontend (CI/CD normal)
2. Webhook desde la Go API cuando se crea/actualiza un build (para regenerar paginas estaticas con datos frescos)

---

## 9. DEPENDENCIAS (package.json estimado)

```json
{
  "dependencies": {
    "astro": "^5.18.0",
    "@astrojs/preact": "^4.0.0",
    "@astrojs/cloudflare": "^12.0.0",
    "preact": "^10.25.0",
    "@preact/signals": "^2.0.0",
    "fuse.js": "^7.0.0"
  },
  "devDependencies": {
    "satori": "^0.12.0",
    "sharp": "^0.33.0",
    "typescript": "^5.7.0"
  }
}
```

**Total de dependencias directas: 6 runtime + 3 dev.** Intencionalmente minimalista.

---

## 10. DECISIONES DESCARTADAS (y por que)

| Descartado | Razon |
|-----------|-------|
| Tailwind CSS | Anade 15KB+ de runtime, PostCSS dependency, utility classes noise en HTML. CSS vanilla con @layer + container queries es mas limpio y performante en 2026. |
| React para islands | 45KB+ runtime. Preact hace lo mismo en 3KB. |
| MDX/Markdown local | El contenido vive en la Go API/SQLite, no en archivos locales. Content Layer con custom loaders es el approach correcto. |
| astro-i18n-aut | Dependencia externa para algo que se resuelve con ~50 lineas de helper + file structure. Menos deps = menos superficie de rotura. |
| SvelteKit | Excelente framework pero StackRigs es content-driven, no app-driven. Astro esta optimizado exactamente para este caso de uso. |
| Astro 6 beta | Tentador por el workerd dev server, pero beta != produccion. Migrar sera straightforward cuando 6.0 sea estable. |
| SSR para todas las paginas | Innecesario. El contenido cambia en eventos discretos (nuevo build, update). Static con rebuilds on-demand es mas rapido y barato. |
| Pagefind | Buena alternativa a Fuse.js para busqueda statica, pero StackRigs tiene busqueda cross-entity (builds + builders + technologies) que se beneficia del control fino de Fuse.js. |

---

## 11. RESUMEN EJECUTIVO

StackRigs frontend es un sitio Astro 5.18 de output hibrido desplegado en Cloudflare Pages. El 95% del sitio es HTML estatico generado en build time desde la Go API via Content Layer loaders. Solo dos islands de Preact aportan interactividad: busqueda fuzzy y metricas en tiempo real. El diseno "Editorial Zen" usa tipografia DM y una paleta warm off-white implementada en CSS vanilla con container queries, @layer, y light-dark(). View Transitions proveen navegacion fluida. Speculation Rules API habilita prefetch inteligente. El budget total de JavaScript es ~14KB en la pagina mas pesada. Core Web Vitals perfectos por construccion, no por optimizacion retroactiva.
