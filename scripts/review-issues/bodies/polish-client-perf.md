**Priority:** P3-polish

Batched low-priority cleanup. Source: [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md).

Client perf items that are individually small but add up. None are user-visible breakages; they are cleanup that reduces re-render and network waste.

---

### [P3.1](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.1) Leftover debug log on every map move

- [ ] **Delete the `console.log`** — `client/src/components/map/MapView.tsx:254`: `console.log("zoom:", map.getZoom())` inside `moveend`. Also confirms `setBbox` fires on programmatic moves (see P2.2).

### [P3.2](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.2) Map silently jumps to the user's location on load

- [ ] **Gate geolocation behind user intent** — `client/src/components/map/MapView.tsx:277-297`: `useGeolocation` fires on mount and `map.jumpTo(...)` on success, yanking the map away from the default Hyderabad center for every visitor with no prompt. Gate behind the existing `LocateButton`, or keep the default center until the user acts.

### [P3.3](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.3) usePins duplicates useAsyncData and has no retry

- [ ] **Reimplement on the existing abstraction** — `client/src/hooks/usePins.ts:26-49` hand-rolls the abort / stale-guard / loading logic that `useAsyncData` already provides, and omits `retry`. Replace the body with `useAsyncData`.

### [P3.4](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.4) usePins leaves loading stuck true when bbox goes null

- [ ] **Reset `loading` in the `!bbox` branch** — `client/src/hooks/usePins.ts:18-24, 27`. Same issue in `useAsyncData` when `enabled` flips `true → false`.

### [P3.5](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.5) useCollections.addPin optimistically bumps pin_count even when the pin was already present

- [ ] **Reconcile from the response** — `client/src/hooks/useCollections.ts`. The server does `ON CONFLICT DO NOTHING`; the local count drifts permanently.

### [P3.6](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.6) useSavedStatus fetches the entire favorite ID list to check one pin

- [ ] **Guard with a shared cache** — `client/src/hooks/useFavorites.ts`. Acceptable for one panel; if it's ever called per-pin in a grid it's N requests.

### [P3.7](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.7) useSessionRefresh fires GET /users/:id on every page navigation

- [ ] **Add a TTL** — `client/src/hooks/useSessionRefresh.ts:15-28`, called from **both** `Navbar.tsx:28` and `MapNavBar.tsx:40`. Add a ~5 min TTL or a `sessionStorage` timestamp.

### [P3.16](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md#p3.16) SearchBar sets loading=false from an aborted request's finally

- [ ] **Guard `finally` on `signal?.aborted`** — `SearchBar.tsx:64-70`. Otherwise the dropdown flashes "No results found" during the 300 ms debounce.

---

- [ ] Each item above addressed or explicitly deferred with a comment
- [ ] Lint + typecheck still clean

<sub>Filed automatically from the code review. See [`CODE_REVIEW.md`](https://github.com/jojianya/sweetspot/blob/development/docs/CODE_REVIEW.md) for the full prioritized list.</sub>
