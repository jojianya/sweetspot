import { API_BASE_URL } from "./client";
import { pinEventSchema, pinRemovedEventSchema } from "./schemas";
import type { PinEvent, PinRemovedEvent } from "./schemas";

/**
 * Opens a Server-Sent Events stream of newly created pins within a bbox
 * (optionally filtered by category). The caller must close the EventSource.
 * The stream is public and reconnects automatically on network hiccups.
 *
 * The same connection also carries `pin_removed` events on the separate
 * `goodspot:pin-removed` Redis channel (surfaced as `event: pin_removed`).
 * `onPinRemoved` is optional so existing callers keep working.
 */
export function openPinStream(
  bbox: string,
  category: number | null,
  onPin: (pin: PinEvent) => void,
  onPinRemoved?: (ev: PinRemovedEvent) => void
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
  if (onPinRemoved) {
    es.addEventListener("pin_removed", (raw) => {
      try {
        const data = JSON.parse((raw as MessageEvent).data);
        const ev = pinRemovedEventSchema.parse(data);
        onPinRemoved(ev);
      } catch (err) {
        console.error("[SSE] Failed to parse pin_removed event:", err, (raw as MessageEvent).data);
      }
    });
  }
  return es;
}
