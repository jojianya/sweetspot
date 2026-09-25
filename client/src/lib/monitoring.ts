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

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8081";

const MAX_EVENTS = 100;
const DEDUPE_MS = 10_000;

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
    url: typeof window !== "undefined" ? window.location.href : undefined,
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