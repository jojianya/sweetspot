"use client";

import { useCallback, useEffect, useState } from "react";
import { fetchFavoriteIDs, fetchFavorites, type FavoriteEntry } from "@/lib/api";
import { errorMessage } from "@/lib/utils";

/** Server state for the saved-pins list: entries, loading, error, retry. */
export function useFavorites() {
  const [entries, setEntries] = useState<FavoriteEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    fetchFavorites()
      .then((data) => {
        if (!cancelled) setEntries(data);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [attempt]);

  const retry = useCallback(() => {
    setError(null);
    setLoading(true);
    setAttempt((n) => n + 1);
  }, []);

  return { entries, loading, error, retry };
}

/**
 * Whether the given pin is saved by the current user. Fetches the favorite
 * ID list whenever `enabled` (i.e. the user is logged in) or the pin changes.
 * `setSaved` lets the caller apply optimistic updates.
 */
export function useSavedStatus(pinId: string, enabled: boolean) {
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    fetchFavoriteIDs()
      .then((ids) => {
        if (!cancelled) setSaved(ids.includes(pinId));
      })
      .catch(() => {
        // saved state stays false if the lookup fails
      });
    return () => {
      cancelled = true;
    };
  }, [enabled, pinId]);

  return { saved, setSaved };
}
