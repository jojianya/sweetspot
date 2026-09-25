"use client";

import { useEffect, useRef } from "react";
import { registerPinView } from "@/lib/api";

/**
 * Registers a pin view once per pin id, fire-and-forget. The read endpoints
 * never increment views, so a "view" only counts when the caller actually
 * opens a pin's detail (the map panel, the permalink page, …). The ref guard
 * keeps re-renders and re-selects of the same pin from double counting.
 */
export function usePinView(pinId: string | null | undefined) {
  const registeredRef = useRef<string | null>(null);

  useEffect(() => {
    if (!pinId || registeredRef.current === pinId) return;
    registeredRef.current = pinId;
    registerPinView(pinId).catch(() => {
      // view tracking is best-effort; never surface failures
    });
  }, [pinId]);
}