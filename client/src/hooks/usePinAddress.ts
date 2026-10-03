"use client";

import { useEffect, useState } from "react";
import { reverseGeocode } from "@/lib/api/geocoding";

/**
 * Reverse-geocoded address for a parsed pin coordinate. Fire-and-forget with
 * an AbortController per point: a superseded or unmounted lookup never writes
 * state, and failures resolve to null so callers fall back to a neutral
 * label. Extracted unchanged from PinDetailPanel and PinPageClient.
 */
export function usePinAddress(point: { lat: number; lng: number } | null): string | null {
  const [address, setAddress] = useState<string | null>(null);

  useEffect(() => {
    if (!point) return;
    const controller = new AbortController();
    reverseGeocode(point, controller.signal)
      .then((text) => {
        if (!controller.signal.aborted) setAddress(text);
      })
      .catch(() => {
        // lookup failed; the address row falls back to a neutral label
      });
    return () => controller.abort();
  }, [point]);

  return address;
}
