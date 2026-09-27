import type { NextConfig } from "next";

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

const nextConfig: NextConfig = {
  output: "standalone",
  // The app is accessed at http://127.0.0.1:3000 in local dev; without this
  // Next 16 blocks the HMR WebSocket (/_next/hmr) as a cross-origin dev
  // resource, which shows up as failed ws://127.0.0.1:3000/_next/hmr
  // connections in the browser console.
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  async rewrites() {
    return [
      {
        // The browser calls the API at /api/* on its own origin; this forwards
        // to the Go server and the destination path is masked, so the client
        // sees a normal same-origin request and a normal same-origin Set-Cookie.
        source: "/api/:path*",
        destination: `${apiProxyTarget()}/:path*`,
      },
    ];
  },
};

export default nextConfig;
