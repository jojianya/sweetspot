<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Project conventions

- House rule: never use `dangerouslySetInnerHTML` (or any raw-HTML rendering) on user-generated content — captions, usernames, socials JSON. React escapes by default; only breaks if you explicitly opt out of it. Just don't.

# Source layout (`src/`)

Feature-based. A feature owns its components, hooks, API modules, helpers
and types; anything used by 2+ features lives in `shared/`.

```
src/
├── app/                  # routes only; pages stay thin and import from features
├── features/
│   ├── auth/             # session, login/register/forms, auth store
│   ├── map/              # map view, search, categories, geolocation, theme
│   ├── pins/             # pin CRUD/detail/permalink, views, share
│   ├── photos/           # picker, lightbox, thumbnails, validation
│   ├── collections/      # collections + favorites (saved pins)
│   ├── comments/
│   ├── social/           # follow, reactions, trending, feed API
│   ├── profile/          # public profiles + editor, users API
│   └── admin/            # reports queue, role console, moderation gate
└── shared/
    ├── ui/ layout/ hooks/ api/ utils/ lib/ types/
```

Each feature has only the subfolders it needs (`components/`, `hooks/`,
`api/`, `lib/`, `store/`) plus an `index.ts` public surface. Tests are
co-located with the file they cover. Types live next to their feature
(`features/<x>/types.ts`) unless 2+ features use them, then
`shared/types/`.

# Import rules (enforced by `eslint.config.mjs`)

- Cross-feature and app-route imports go through the barrel only:
  `@/features/<name>`. Never a deep path (`@/features/pins/api/pins`).
- Inside a feature, use relative imports, never `@/` self-references.
- Pure shared layers (`api`, `utils`, `lib`, `types`, `ui`, `hooks`) must
  not import from `features/` at all.
- `shared/layout` (Navbar) is the one exception: app chrome that renders
  session UI, consuming only the auth public surface.
- `vi.mock` targets keep leaf paths — mocking a barrel blanks its sibling
  exports. The lint rule exempts `*.test.*` for exactly this reason.
- React Server Components (`app/layout.tsx`, `app/page.tsx`,
  `app/pin/[id]/page.tsx`) import leaves, not barrels: a barrel re-exports
  hook modules (rejected in an RSC graph) and, for pins, a `next/headers`
  module (rejected in client graphs). The rule exempts exactly those files.
- Never re-export a `next/headers` (or other server-only) module from a
  feature barrel — it poisons every client graph that reaches the barrel.
- No barrels inside `shared/` (the trimmed `shared/utils` re-export of two
  pure helpers predates this layout and stays).

# Checks

- `pnpm next typegen` first on a cold `.next` (emits route types for tsc),
  then `pnpm tsc --noEmit`, `pnpm lint`, `pnpm vitest run`, `pnpm build`.
