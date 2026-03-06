-- ============================================================================
-- StackRigs — Seed: tecnologias iniciales
-- stackrigs.com
-- ============================================================================
-- 40 tecnologias populares organizadas por categoria.
-- Ejecutar DESPUES de schema.sql.
-- ============================================================================
-- Tabla: technologies (name, slug, category)
-- 'name' es el nombre para mostrar (ej: "Next.js")
-- 'slug' es el identificador URL-safe (ej: "nextjs")
-- ============================================================================

-- Frameworks
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('Next.js',       'nextjs',      'framework'),
    ('Nuxt',          'nuxt',        'framework'),
    ('Remix',         'remix',       'framework'),
    ('Astro',         'astro',       'framework'),
    ('SvelteKit',     'sveltekit',   'framework'),
    ('Express',       'express',     'framework'),
    ('Fastify',       'fastify',     'framework'),
    ('Hono',          'hono',        'framework'),
    ('Django',        'django',      'framework'),
    ('Ruby on Rails', 'rails',       'framework'),
    ('Laravel',       'laravel',     'framework'),
    ('Spring Boot',   'spring-boot', 'framework');

-- Languages
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('TypeScript',    'typescript',  'language'),
    ('JavaScript',    'javascript',  'language'),
    ('Python',        'python',      'language'),
    ('Rust',          'rust',        'language'),
    ('Go',            'go',          'language'),
    ('Ruby',          'ruby',        'language'),
    ('Java',          'java',        'language');

-- Databases
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('SQLite',        'sqlite',      'database'),
    ('PostgreSQL',    'postgresql',  'database'),
    ('MySQL',         'mysql',       'database'),
    ('MongoDB',       'mongodb',     'database'),
    ('Redis',         'redis',       'database'),
    ('Turso',         'turso',       'database'),
    ('Supabase',      'supabase',    'database');

-- Hosting
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('Vercel',        'vercel',      'hosting'),
    ('Cloudflare',    'cloudflare',  'hosting'),
    ('Fly.io',        'fly-io',      'hosting'),
    ('Railway',       'railway',     'hosting'),
    ('AWS',           'aws',         'hosting'),
    ('Hetzner',       'hetzner',     'hosting');

-- CDN
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('Cloudflare CDN', 'cloudflare-cdn', 'cdn'),
    ('Bunny CDN',      'bunny-cdn',      'cdn');

-- CI/CD
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('GitHub Actions', 'github-actions', 'ci_cd'),
    ('GitLab CI',      'gitlab-ci',      'ci_cd');

-- Monitoring
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('Sentry',        'sentry',      'monitoring'),
    ('Better Stack',  'betterstack', 'monitoring');

-- AI
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('OpenAI',        'openai',      'ai'),
    ('Anthropic',     'anthropic',   'ai'),
    ('Ollama',        'ollama',      'ai');

-- Tools
INSERT OR IGNORE INTO technologies (name, slug, category) VALUES
    ('Docker',        'docker',      'tool'),
    ('Tailwind CSS',  'tailwindcss', 'tool'),
    ('Prisma',        'prisma',      'tool'),
    ('Drizzle ORM',   'drizzle',     'tool'),
    ('React',         'react',       'tool'),
    ('Svelte',        'svelte',      'tool'),
    ('Vue',           'vue',         'tool'),
    ('htmx',          'htmx',        'tool');
