import { API_BASE_URL } from "./client";
import { pinListEntrySchema } from "./schemas";
import type { PinListEntry } from "@/lib/types";

/**
 * Opens a Server-Sent Events stream of newly created pins within a bbox
 * (optionally filtered by category). The caller must close the EventSource.
 * The stream is public and reconnects automatically on network hiccups.
 */
export function openPinStream(
  bbox: string,
  category: number | null,
  onPin: (pin: PinListEntry) => void
): EventSource {
  const params = new URLSearchParams({ bbox });
  if (category !== null) params.set("category", String(category));

  const es = new EventSource(`${API_BASE_URL}/events?${params.toString()}`);
  es.addEventListener("pin", (raw) => {
    try {
      const pin = pinListEntrySchema.parse(JSON.parse((raw as MessageEvent).data));
      onPin(pin);
    } catch {
      // Malformed or unexpected payloads are skipped.
    }
  });
  return es;
}