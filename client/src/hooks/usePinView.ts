"use client";

import { useEffect, useRef, useState } from "react";
import { registerPinView } from "@/lib/api";
import { useAuth } from "@/store/auth";

export interface PinViewResult {
  pinId: string;
  views: number;
}

/**
 * Registers one view when a pin's detail opens and returns the response
 * count. Returns null whenever no request was sent: logged-out visitors
 * (the server wouldn't count them), the owner's own opens, the once-per-pin
 * repeat guard, and failed calls (logged at debug level, never surfaced).
 * Mount from detail surfaces only — never lists or SSR.
 */
export function usePinView(
  pinId: string | null | undefined,
  ownerId?: string | null
): PinViewResult | null {
  const registeredRef = useRef<string | null>(null);
  const { user } = useAuth();
  const [result, setResult] = useState<PinViewResult | null>(null);

  useEffect(() => {
    if (!pinId || !user) return;
    if (ownerId && user.id === ownerId) return;
    if (registeredRef.current === pinId) return;
    registeredRef.current = pinId;
    let cancelled = false;
    registerPinView(pinId)
      .then((views) => {
        if (!cancelled) setResult({ pinId, views });
      })
      .catch(() => {
        console.debug("[pin-view] registration failed");
      });
    return () => {
      cancelled = true;
    };
  }, [pinId, ownerId, user]);

  return result;
}
