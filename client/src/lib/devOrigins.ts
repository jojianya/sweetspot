/**
 * Extra origins allowed to request the Next.js dev server.
 *
 * Next blocks cross-origin requests to dev-only assets and endpoints (notably
 * the HMR WebSocket at /_next/hmr) unless the requesting hostname is listed in
 * `allowedDevOrigins`. That means opening the app from a LAN address such as
 * http://192.168.68.112:3000 fails with a blocked cross-origin error until the
 * address is allowed, which previously meant hardcoding one machine's IP into
 * next.config.ts and editing it every time the network changed.
 *
 * The list is read from NEXT_ALLOWED_DEV_ORIGINS as a comma-separated string.
 * It is deliberately not NEXT_PUBLIC_*: this is consumed by the dev server, not
 * inlined into the browser bundle, so there is no reason to expose it.
 */

/**
 * Origins Next allows by default. Both are listed explicitly because Next's own
 * default is `localhost` alone, and the app is routinely opened via the
 * loopback IP instead.
 */
export const DEFAULT_DEV_ORIGINS = ["localhost", "127.0.0.1"] as const;

/**
 * Reduces one configured entry to the bare hostname Next matches against.
 *
 * The documented format is a hostname or a `*.suffix` wildcard, so an entry
 * carrying a scheme, port, path or trailing slash would silently never match.
 * Those are stripped rather than rejected: this value is a developer
 * convenience, and failing `next dev` outright over a stray slash is a worse
 * outcome than quietly accepting the hostname it obviously meant.
 */
export function normalizeDevOrigin(raw: string): string | null {
  const trimmed = raw.trim();
  if (!trimmed) return null;

  // Drop a scheme if present. `new URL` needs one to parse, and the wildcard
  // form (`*.example.dev`) is not a URL, so only attempt parsing when there is
  // no leading wildcard.
  let candidate = trimmed;
  if (!candidate.startsWith("*")) {
    try {
      const url = new URL(candidate.includes("://") ? candidate : `http://${candidate}`);
      // `url.host` keeps a non-default port; Next matches the hostname, so drop it.
      candidate = url.hostname;
    } catch {
      // Not parseable as a URL. Fall through and use the raw value: a bare
      // hostname like `box.local` parses as http://box.local anyway, and this
      // keeps genuinely odd entries (internal TLDs) working.
    }
  }

  const normalized = candidate.toLowerCase().replace(/\/+$/, "").trim();
  return normalized || null;
}

/**
 * Builds the `allowedDevOrigins` list from a comma-separated value.
 *
 * Falls back to DEFAULT_DEV_ORIGINS when the variable is unset, empty, or
 * contains nothing but separators, so `next build` and CI work with no
 * configuration at all.
 */
export function resolveAllowedDevOrigins(
  configured = process.env.NEXT_ALLOWED_DEV_ORIGINS
): string[] {
  const seen = new Set<string>();
  for (const entry of (configured ?? "").split(",")) {
    const origin = normalizeDevOrigin(entry);
    if (origin !== null) seen.add(origin);
  }

  // Set iteration order is insertion order, so the configured order is kept.
  return seen.size > 0 ? [...seen] : [...DEFAULT_DEV_ORIGINS];
}