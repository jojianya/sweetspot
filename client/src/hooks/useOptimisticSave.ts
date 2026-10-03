"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { removeFavorite, saveFavorite } from "@/lib/api";
import { useSavedStatus } from "@/hooks/useFavorites";
import { useAuth } from "@/store/auth";

/**
 * Optimistic save toggle for a pin. Logged-out callers are sent to /login;
 * logged-in callers flip `saved` immediately and roll back if the API fails.
 * The in-flight guard drops re-entrant clicks. Extracted unchanged from
 * PinDetailPanel and PinPageClient (both used the same sequence).
 */
export function useOptimisticSave(pinId: string) {
  const router = useRouter();
  const { user } = useAuth();
  const { saved, setSaved } = useSavedStatus(pinId, user !== null);
  const [saving, setSaving] = useState(false);
  // Ref-based in-flight guard: `saving` only updates on the next render, so
  // two clicks in the same tick would both pass the state check and send
  // duplicate requests. The ref flips synchronously instead.
  const busyRef = useRef(false);

  const handleSave = () => {
    if (!user) {
      router.push("/login");
      return;
    }
    if (saving || busyRef.current) return;
    busyRef.current = true;
    setSaving(true);
    const next = !saved;
    setSaved(next);
    const op = next ? saveFavorite(pinId) : removeFavorite(pinId);
    const done = () => {
      busyRef.current = false;
      setSaving(false);
    };
    op.then(done).catch(() => {
      setSaved(!next);
      done();
    });
  };

  return { saved, saving, handleSave };
}
