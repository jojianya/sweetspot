/**
 * Client-side error reporting.
 *
 * Crashes caught by the ErrorBoundary, unhandled window errors/rejections, and
 * 5xx API failures are throttled and POSTed to the server's /errors ingest
 * endpoint, where they flow through the same reporter as server errors (slog
 * logs, plus Sentry once SENTRY_DSN is configured on the backend).
 *
 * Reporting is deliberately best-effort and fail-silent: it never throws and
 * never blocks or floods the app.
 */

/**
 * Base path the browser uses for every API call.
 *
 * Relative on purpose. next.config.ts rewrites /api/* to the Go server, so the
 * browser only ever talks to its own origin and the session cookie is
 * first-party. That is what makes the cookie's SameSite=Strict workable: with
 * the old absolute NEXT_PUBLIC_API_URL the API was a different origin from the
 * page, so loading the app from 127.0.0.1:3000 while the API sat on
 * localhost:8081 made every authenticated request cross-site, the browser
 * withheld the cookie without warning, and each protected page answered 401 —
 * which the client read as an ended session and logged the user out.
 *
 * NEXT_PUBLIC_API_URL still exists for two unrelated jobs and must not be
 * confused with this one: absolute URLs in SSR metadata (lib/site.ts) and the
 * SSR fetch fallback (lib/api/server.ts).
 */
export const API_BASE_URL = "/api";

const MAX_EVENTS = 100;
const DEDUPE_MS = 10_000;

/**
 * The page location with its query string and fragment removed.
 *
 * A URL can carry a token, an invite code or a signed link in those parts, and
 * this payload is POSTed to the error ingest endpoint and then into logs and
 * Sentry. Only the origin and path are needed to locate the failure.
 * window.location is used rather than the URL constructor because a malformed
 * href would throw inside the reporter; this is best-effort by design.
 */
function safePageUrl(): string | undefined {
  if (typeof window === "undefined") return undefined;
  const { origin, pathname } = window.location;
  return `${origin}${pathname}`;
}

let sent = 0;
const lastSeen = new Map<string, number>();

export function reportError(
  error: unknown,
  context?: Record<string, unknown>,
): void {
  const message = error instanceof Error ? error.message : String(error);
  // Dedupe by kind+url+message so a failing endpoint is throttled (same key)
  // while genuinely distinct failures still surface.
  const key = `${context?.kind ?? ""}|${context?.url ?? ""}|${message}`;

  if (sent >= MAX_EVENTS) return;
  const now = Date.now();
  const previous = lastSeen.get(key) ?? Number.NEGATIVE_INFINITY;
  if (now - previous < DEDUPE_MS) return;
  lastSeen.set(key, now);
  sent += 1;

  const payload = {
    message,
    stack: error instanceof Error ? error.stack : undefined,
    url: safePageUrl(),
    extra: context ?? {},
  };

  try {
    void fetch(`${API_BASE_URL}/errors`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
      keepalive: true,
    }).catch(() => undefined);
  } catch {
    // Reporting must never break the app.
  }
}

let installed = false;

/** Idempotently registers window-level error and unhandled-rejection handlers. */
export function initGlobalErrorReporting(): void {
  if (installed || typeof window === "undefined") return;
  installed = true;
  window.addEventListener("error", (event) => {
    reportError(event.error ?? new Error(event.message), {
      kind: "window.error",
    });
  });
  window.addEventListener("unhandledrejection", (event) => {
    reportError(event.reason, { kind: "unhandledrejection" });
  });
}

/** Test-only: resets the throttling counters so tests can reason about them. */
export function _resetForTests(): void {
  sent = 0;
  lastSeen.clear();
}