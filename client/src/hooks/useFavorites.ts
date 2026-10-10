"use client";

import { useEffect, useState } from "react";
import { fetchFavorites, fetchIsSaved, type FavoriteEntry } from "@/lib/api";
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
 * Whether the given pin is saved by the current user. Queries the server's
 * per-pin check (`GET /favorites/:id/saved`, one indexed EXISTS probe) rather
 * than pulling the whole id list, which is capped at 5000 server-side and is
 * only meant to be fetched as a membership set.
 * `setSaved` lets the caller apply optimistic updates.
 */
export function useSavedStatus(pinId: string, enabled: boolean) {
  const [saved, setSaved] = useState(false);

  // Reset during render when the identity changes, matching useAsyncData's
  // derived-state pattern: the previous pin's saved badge must not linger on
  // the new pin, and disabling the hook must not keep the last answer. Done in
  // render (not an effect) so the very first paint for a new pin is already
  // correct, with no one-frame flash of the old value.
  const [prevKey, setPrevKey] = useState(`${pinId}:${enabled}`);
  const key = `${pinId}:${enabled}`;
  if (prevKey !== key) {
    setPrevKey(key);
    setSaved(false);
  }

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    fetchIsSaved(pinId)
      .then((res) => {
        if (!cancelled) setSaved(res);
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