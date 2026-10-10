"use client";

import { useEffect } from "react";
import { openPinStream } from "@/lib/api";
import type { PinEvent } from "@/lib/api/schemas";
import type { PinListEntry } from "@/lib/types/pin";

/**
 * Subscribes to the realtime stream of new pins for the current view.
 * `bbox`/`category` mirror the map's live query; `onPin` must be referentially
 * stable (e.g. the usePins `addPin` callback) to avoid reconnecting per render.
 *
 * The SSE stream emits a lightweight `PinEvent`; we enrich it to `PinListEntry`
 * with defaults for fields the backend doesn't include in the live payload.
 *
 * Removals arrive on the same connection as `event: pin_removed` (separate
 * `goodspot:pin-removed` Redis channel, never mixed into `goodspot:pins`).
 * `onPinRemoved` receives the removed pin id so the map list can drop it and
 * the detail panel can close when its open pin disappears.
 *
 * Reconnect catch-up: SSE has no replay, so the refetch that matters is the
 * one after the stream is live again. `onerror` only marks the episode —
 * there is no timer refetch while the stream is still broken, since it could
 * be immediately stale. The first `onopen` after any error (the browser's
 * successful reconnect) ends the episode and refetches immediately, so an
 * episode produces exactly one refetch if a reconnect follows, zero otherwise.
 * The flag is scoped to this `EventSource` instance — creating a new one for
 * an intentional re-subscription (bbox/category change) clears it, so
 * intentional resubs never refetch.
 */
export function usePinStream(
  bbox: string | null,
  category: number | null,
  onPin: (pin: PinListEntry) => void,
  onPinRemoved?: (pinId: string) => void,
  onReconnect?: () => void
) {
  useEffect(() => {
    if (!bbox) return;
    let cancelled = false;
    let sawError = false;

    const es = openPinStream(
      bbox,
      category,
      (event: PinEvent) => {
        // Enrich the minimal SSE payload to the full PinListEntry shape
        const pin: PinListEntry = {
          id: event.id,
          user_id: event.user_id ?? "",
          location: event.location,
          geohash: "", // Not provided by SSE; will be backfilled on next full fetch
          caption: event.caption,
          category_id: event.category_id,
          is_hidden: false, // New pins are never hidden
          // Safe only because the server never publishes a hidden pin on the
          // `pin` channel: CreatePin's INSERT leaves is_hidden at its false
          // default (pins/repository.go), UpdatePin publishes nothing at all,
          // and pins.Event has no is_hidden field, so the channel cannot
          // express "hidden". A pin that becomes hidden is withdrawn through
          // the separate pin_removed event, which usePinStream's onPinRemoved
          // drops from the list. If a pin-update event is ever added, this
          // value must come from that event instead of staying hardcoded.
          views: 0, // New pins start at 0 views
          created_at: event.created_at,
          cover_url: event.cover_url ?? "",
          username: null, // Not provided by SSE; will be backfilled on next full fetch
        };
        onPin(pin);
      },
      onPinRemoved ? (ev) => onPinRemoved(ev.id) : undefined
    );

    es.onerror = () => {
      // Mark the episode. No timer: a refetch while the stream is still
      // broken could be immediately stale (SSE has no replay). The catch-up
      // refetch happens on the first onopen after the error instead.
      sawError = true;
    };

    es.onopen = () => {
      // First open after any error is the successful reconnect: consume the
      // episode and catch up immediately, even if time has passed since the
      // error. A clean open with no prior error is a no-op.
      if (!sawError) return;
      sawError = false;
      if (!cancelled) onReconnect?.();
    };

    return () => {
      cancelled = true;
      es.close();
    };
  }, [bbox, category, onPin, onPinRemoved, onReconnect]);
}
