"use client";

import { useState } from "react";
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

  const handleSave = () => {
    if (!user) {
      router.push("/login");
      return;
    }
    if (saving) return;
    setSaving(true);
    const next = !saved;
    setSaved(next);
    const op = next ? saveFavorite(pinId) : removeFavorite(pinId);
    op.then(() => setSaving(false)).catch(() => {
      setSaved(!next);
      setSaving(false);
    });
  };

  return { saved, saving, handleSave };
}
