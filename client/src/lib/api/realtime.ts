import { API_BASE_URL } from "./client";
import { pinEventSchema } from "./schemas";
import type { PinEvent } from "./schemas";

/**
 * Opens a Server-Sent Events stream of newly created pins within a bbox
 * (optionally filtered by category). The caller must close the EventSource.
 * The stream is public and reconnects automatically on network hiccups.
 */
export function openPinStream(
  bbox: string,
  category: number | null,
  onPin: (pin: PinEvent) => void
): EventSource {
  const params = new URLSearchParams({ bbox });
  if (category !== null) params.set("category", String(category));

  const es = new EventSource(`${API_BASE_URL}/events?${params.toString()}`);
  es.addEventListener("pin", (raw) => {
    try {
      const data = JSON.parse((raw as MessageEvent).data);
      const pin = pinEventSchema.parse(data);
      onPin(pin);
    } catch (err) {
      console.error("[SSE] Failed to parse pin event:", err, (raw as MessageEvent).data);
    }
  });
  return es;
}