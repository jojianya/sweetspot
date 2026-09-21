import { pinDetailSchema } from "./schemas";
import type { PinDetail } from "@/lib/types";

/**
 * Internal API base used for server-side (SSR) fetches. Inside Docker the
 * client container reaches the API via the compose service name; in plain
 * local dev the browser-visible localhost URL works.
 */
const SSR_API_URL =
  process.env.API_INTERNAL_URL ??
  process.env.NEXT_PUBLIC_API_URL ??
  "http://localhost:8081";

export async function fetchPinServer(id: string): Promise<PinDetail | null> {
  try {
    const res = await fetch(`${SSR_API_URL}/pins/${id}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    if (!res.ok) return null;
    const json = (await res.json()) as { pin?: unknown };
    return pinDetailSchema.parse(json.pin);
  } catch {
    // Server unavailable during SSR: the page falls back to a client fetch.
    return null;
  }
}