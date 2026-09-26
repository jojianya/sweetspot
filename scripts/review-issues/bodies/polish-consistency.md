**Priority:** P3-polish

Batched low-priority cleanup. Source: [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md).

Small mismatches between duplicated logic — client/server validation limits, unreachable branches, overly broad auth clearing — plus two places on the profile page that should degrade gracefully instead of blanking out.

---

### [P3.9](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.9) Client caption limit (200) ≠ server limit (500)

- [ ] **Pick one number** — `CreatePinButton.tsx:284`, `PinEditSheet.tsx:114` vs `internal/modules/pins/dto.go`. Export it from a shared constant.

### [P3.11](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.11) pin-default icon is unreachable dead code

- [ ] **Delete the branch or make categories dynamic** — `pinLayers.ts:285, 374` vs `CATEGORY_IDS = [1..8]` (`:62`) and `DOT_COLORS[categoryId % len]`: every id in 1..8 always matches a `pin-cat-N` image, so `pin-default` never renders. Make `CATEGORY_IDS` derive from the API's `/categories` response so new DB categories get icons without a code change.

### [P3.14](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.14) users/[id] uses Promise.all for profile + pins + collections

- [ ] **Use `Promise.allSettled`** — `page.tsx:133-137`. A single failed `fetchUserPins` blanks the whole profile. Render partial results instead.

### [P3.15](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.15) users/[id] return null (blank screen, no Navbar)

- [ ] **Return an error UI instead of `null`** — `page.tsx:293`. Unreachable today but has no fallback.

### [P3.20](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.20) Auth is cleared on *any* 401

- [ ] **Whitelist paths instead of blacklisting one** — `client/src/lib/api/client.ts:31-36`. If a future anonymous endpoint 401s, every user gets logged out.

---

- [ ] Each item above addressed or explicitly deferred with a comment
- [ ] Lint + typecheck still clean

<sub>Filed automatically from the code review. See [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md) for the full prioritized list.</sub>
