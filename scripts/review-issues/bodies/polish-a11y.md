**Priority:** P3-polish

Batched low-priority cleanup. Source: [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md).

Accessibility and keyboard-navigation gaps. `useDialogFocus` already exists in the codebase, so most of these are "use the helper that is already there" rather than new work.

---

### [P3.8](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.8) useDialogFocus has no body scroll lock and no inert

- [ ] **Lock `document.body` overflow while open** — `client/src/hooks/useDialogFocus.ts`.

### [P3.10](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.10) CreatePinButton defaults categoryId to 1 before categories load

- [ ] **Derive from the loaded list** — `CreatePinButton.tsx:43`: `useState<number>(categories[0]?.id ?? 1)`. `useState` never re-runs, so the default is permanently "Food" by accident. Default to `null` and require a selection.

### [P3.12](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.12) roles/page.tsx — pointless IIFE wrapping a single button

- [ ] **Inline it** — `client/src/app/roles/page.tsx:218`.

### [P3.13](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.13) Collections links on the profile page go nowhere

- [ ] **Either filter by it or point at a real route** — `client/src/app/users/[id]/page.tsx:550`: `href={`/users/${id}?collection=${c.id}`}`, but the page never reads the `collection` search param. Currently a dead link.

---

- [ ] Each item above addressed or explicitly deferred with a comment
- [ ] Lint + typecheck still clean

<sub>Filed automatically from the code review. See [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md) for the full prioritized list.</sub>
