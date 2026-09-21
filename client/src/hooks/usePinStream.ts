"use client";

import { useEffect } from "react";
import { openPinStream } from "@/lib/api";
import type { PinListEntry } from "@/lib/types";

/**
 * Subscribes to the realtime stream of new pins for the current view.
 * `bbox`/`category` mirror the map's live query; `onPin` must be referentially
 * stable (e.g. the usePins `addPin` callback) to avoid reconnecting per render.
 */
export function usePinStream(
  bbox: string | null,
  category: number | null,
  onPin: (pin: PinListEntry) => void
) {
  useEffect(() => {
    if (!bbox) return;
    const es = openPinStream(bbox, category, onPin);
    return () => es.close();
  }, [bbox, category, onPin]);
}