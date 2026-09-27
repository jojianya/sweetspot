import { cache } from "react";
import { headers } from "next/headers";
import { pinDetailSchema } from "./schemas";
import type { PinDetail } from "@/lib/types";

/**
 * Internal API base used for server-side (SSR) fetches.
 *
 * API_INTERNAL_URL is the correct value in Docker (the compose service name,
 * e.g. http://server:8081). NEXT_PUBLIC_API_URL is the browser-facing URL
 * and must NOT be used for SSR — inside the client container, localhost is
 * the container itself, not the host.
 *
 * When API_INTERNAL_URL is unset we fall back to NEXT_PUBLIC_API_URL but log
 * a warning so the misconfiguration is visible. In production we throw
 * instead, because a silent fallback there means SSR pages break at deploy
 * time while CI stays green.
 *
 * Resolved lazily inside fetchPinServer so the module can be imported during
 * build (when env vars may be incomplete) without throwing.
 */
const resolveSSRUrl = (): string => {
  const internal = process.env.API_INTERNAL_URL;
  if (internal) return internal.replace(/\/+$/, "");

  const fallback = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8081";
  const isProd = process.env.NODE_ENV === "production";

  if (isProd) {
    throw new Error(
      "API_INTERNAL_URL is not set. SSR fetches cannot use the browser-facing " +
        "URL inside the client container. Set API_INTERNAL_URL to the compose " +
        "service name (e.g. http://server:8081)."
    );
  }

  console.warn(
    "[server.ts] API_INTERNAL_URL is not set; falling back to NEXT_PUBLIC_API_URL " +
      `(${fallback}). This is wrong in Docker — set API_INTERNAL_URL to the compose ` +
      "service name for SSR fetches."
  );
  return fallback.replace(/\/+$/, "");
};

export async function fetchPinServer(id: string): Promise<PinDetail | null> {
  // Forward the incoming cookie so the API can authenticate the SSR request
  // the same way it authenticates browser requests. Without this, an owner
  // requesting their own hidden pin over SSR gets a 404 because the request
  // arrives unauthenticated.
  const cookie = (await headers()).get("cookie") ?? "";
  const ssrUrl = resolveSSRUrl();

  const response = await fetch(`${ssrUrl}/pins/${id}`, {
    cache: "no-store",
    signal: AbortSignal.timeout(5000),
    headers: cookie ? { cookie } : {},
  });

  if (response.status === 404) return null;
  if (!response.ok) {
    throw new Error(`Pin API returned HTTP ${response.status}`);
  }

  const json = (await response.json()) as { pin?: unknown };
  return pinDetailSchema.parse(json.pin);
}

/** Share one API read between generateMetadata and the page in each SSR request. */
export const getPinServer = cache(fetchPinServer);
