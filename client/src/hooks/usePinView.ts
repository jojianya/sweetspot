"use client";

import { useEffect, useRef } from "react";
import { registerPinView } from "@/lib/api";
import { useAuth } from "@/store/auth";

/**
 * Registers one view when a pin's detail opens. Skips logged-out visitors
 * (the server wouldn't count them) and the owner's own opens (unique views
 * are per other-account). The ref guard keeps re-renders and re-selects of
 * the same pin from sending repeats; failures stay at debug level and never
 * surface. Mount from detail surfaces only — never lists or SSR.
 */
export function usePinView(pinId: string | null | undefined, ownerId?: string | null) {
  const registeredRef = useRef<string | null>(null);
  const { user } = useAuth();

  useEffect(() => {
    if (!pinId || !user) return;
    if (ownerId && user.id === ownerId) return;
    if (registeredRef.current === pinId) return;
    registeredRef.current = pinId;
    registerPinView(pinId).catch(() => {
      console.debug("[pin-view] registration failed");
    });
  }, [pinId, ownerId, user]);
}
