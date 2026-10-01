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
 */
export function usePinStream(
  bbox: string | null,
  category: number | null,
  onPin: (pin: PinListEntry) => void
) {
  useEffect(() => {
    if (!bbox) return;
    const es = openPinStream(bbox, category, (event: PinEvent) => {
      // Enrich the minimal SSE payload to the full PinListEntry shape
      const pin: PinListEntry = {
        id: event.id,
        user_id: event.user_id ?? "",
        location: event.location,
        geohash: "", // Not provided by SSE; will be backfilled on next full fetch
        caption: event.caption,
        category_id: event.category_id,
        is_hidden: false, // New pins are never hidden
        views: 0, // New pins start at 0 views
        created_at: event.created_at,
        cover_url: event.cover_url ?? "",
        username: null, // Not provided by SSE; will be backfilled on next full fetch
      };
      onPin(pin);
    });
    return () => es.close();
  }, [bbox, category, onPin]);
}