"use client";

import { useEffect, useState } from "react";
import { fetchFavoriteIDs, fetchFavorites, type FavoriteEntry } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";

/** Matches the server's maximum page size for the saved-pins list. */
const FAVORITES_PAGE_SIZE = 200;

/** Server state for the saved-pins list: entries, loading, error, retry. */
export function useFavorites() {
  const { data, loading, error, retry } = useAsyncData<FavoriteEntry[]>(
    // The endpoint is paginated now, so ask for a page large enough to cover
    // the list and unwrap the {entries, total} envelope.
    async () => (await fetchFavorites({ limit: FAVORITES_PAGE_SIZE })).entries,
    []
  );

  return { entries: data ?? [], loading, error, retry };
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