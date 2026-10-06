import type { NextConfig } from "next";

import { resolveAllowedDevOrigins } from "./src/lib/devOrigins";

/**
 * Server-side address of the Go API, used as the rewrite destination.
 *
 * This is resolved by the Next *server*, never by the browser, so it must be an
 * address the server itself can reach: in Docker that is the compose service
 * name (http://server:8081), and when running `next dev` straight on the host it
 * is the published loopback port. That is exactly what API_INTERNAL_URL is for
 * (see lib/api/server.ts, which uses the same variable for SSR fetches).
 *
 * NEXT_PUBLIC_API_URL is deliberately NOT preferred here. It used to be the
 * browser-facing API origin, and pointing the browser at a different origin than
 * the page is what made the session cookie fragile: the cookie is SameSite=Strict
 * and host-only, so loading the app from 127.0.0.1:3000 while the API sat on
 * localhost:8081 made every authenticated request cross-site, the browser
 * silently withheld the cookie, and each protected page answered 401 — which the
 * client correctly read as an ended session and logged the user out. Proxying
 * /api/* makes the API same-origin, so the cookie is first-party and that whole
 * class of failure is gone regardless of which loopback name is used.
 *
 * NEXT_PUBLIC_API_URL is still meaningful, but only for absolute URLs in SSR
 * metadata (lib/site.ts) and as the SSR fetch fallback (lib/api/server.ts).
 */
function apiProxyTarget(): string {
  const internal = process.env.API_INTERNAL_URL?.trim();
  const target = internal || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
  return target.replace(/\/+$/, "");
}

// Warn once at config load when the production proxy silently falls back to
// the loopback default: every /api, /events and /uploads rewrite then points
// at the Next server itself and historical images 404. Never throws and never
// warns in development.
if (
  process.env.NODE_ENV === "production" &&
  !process.env.API_INTERNAL_URL?.trim() &&
  !process.env.NEXT_PUBLIC_API_URL?.trim()
) {
  console.warn(
    "[next.config] API_INTERNAL_URL and NEXT_PUBLIC_API_URL are unset in production; " +
      "/api, /events and /uploads will proxy to http://localhost:8081. " +
      "Set API_INTERNAL_URL=http://server:8081."
  );
}

const nextConfig: NextConfig = {
  output: "standalone",
  // SSE /events must stream uncompressed: Next's compressor buffers
  // proxied event frames indefinitely when the client sends
  // Accept-Encoding: gzip. nginx handles compression in prod.
  compress: false,
  // Next blocks cross-origin requests to dev-only assets and endpoints, which
  // shows up as a failed ws://<host>:3000/_next/hmr connection. The app is
  // reached by hostname (localhost), by loopback IP (127.0.0.1) and by LAN IP
  // when testing from another device, so the list comes from
  // NEXT_ALLOWED_DEV_ORIGINS (comma-separated) and falls back to the two
  // loopback names. See src/lib/devOrigins.ts.
  allowedDevOrigins: resolveAllowedDevOrigins(),
  async rewrites() {
    return [
      {
        // The browser calls the API at /api/* on its own origin; this forwards
        // to the Go server and the destination path is masked, so the client
        // sees a normal same-origin request and a normal same-origin Set-Cookie.
        source: "/api/:path*",
        destination: `${apiProxyTarget()}/:path*`,
      },
      {
        // SSE /events endpoint for realtime pin updates.
        // Must be proxied so the browser sends the httpOnly session cookie.
        source: "/events",
        destination: `${apiProxyTarget()}/events`,
      },
      {
        // Uploaded pin photos and avatars live on the Go server's /uploads
        // route (see server/internal/http/router.go). The DB stores absolute
        // URLs with the STORAGE_BASE_URL host at upload time, so a LAN-IP
        // change orphans every old URL. The client normalizes all media to
        // same-origin /uploads/... paths (see lib/media.ts), and this rewrite
        // serves them from the Go server without exposing the browser to the
        // backend's host/port at all.
        source: "/uploads/:path*",
        destination: `${apiProxyTarget()}/uploads/:path*`,
      },
    ];
  },
};

export default nextConfig;
