import { cache } from "react";
import { headers } from "next/headers";
import { pinDetailSchema } from "./schemas";
import type { PinDetail } from "@/lib/types";

/**
 * Internal API base used for server-side (SSR) fetches. Inside Docker the
 * client container reaches the API via the compose service name; in plain
 * local dev the browser-visible localhost URL works.
 */
const SSR_API_URL = (
  process.env.API_INTERNAL_URL ??
  process.env.NEXT_PUBLIC_API_URL ??
  "http://localhost:8081"
).replace(/\/+$/, "");

export async function fetchPinServer(id: string): Promise<PinDetail | null> {
  // Forward the incoming cookie so the API can authenticate the SSR request
  // the same way it authenticates browser requests. Without this, an owner
  // requesting their own hidden pin over SSR gets a 404 because the request
  // arrives unauthenticated.
  const cookie = (await headers()).get("cookie") ?? "";

  const response = await fetch(`${SSR_API_URL}/pins/${id}`, {
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
