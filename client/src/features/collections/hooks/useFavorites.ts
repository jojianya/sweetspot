"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useClearOnChange } from "@/shared/hooks/useClearOnChange";
import { fetchFavorites, fetchIsSaved, type FavoriteEntry } from "../api/favorites";
import { errorMessage } from "@/shared/utils";

/** Matches the server's maximum page size for the saved-pins list. */
const FAVORITES_PAGE_SIZE = 200;

/**
 * Server state for the saved-pins list: one page of entries, the account's
 * whole saved count, and a load-more that appends the next page.
 *
 * The endpoint is paginated and the server reports `total`. Discarding it and
 * showing `entries.length` instead made any account with more than one page
 * look like it had exactly 200 saved pins, with no way to reach the rest.
 */
export function useFavorites() {
  const [entries, setEntries] = useState<FavoriteEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Which page to fetch. Offset 0 replaces the list (first page, or a retry);
  // anything else appends. `attempt` is bumped to re-fetch the same page.
  const [offset, setOffset] = useState(0);
  const [attempt, setAttempt] = useState(0);
  // Tag each run so a superseded page can neither overwrite a newer result set
  // nor clear a newer request's loading flags. Without this a "load more" still
  // in flight can land after a retry and replace the fresh first page.
  const fetchRunRef = useRef(0);

  // Derived state, the idiom usePins uses: flip the loading flag during render
  // so no setState runs inside the effect below, which react-hooks forbids.
  const runKey = `${offset}:${attempt}`;
  const [appliedKey, setAppliedKey] = useState<string | null>(null);
  if (appliedKey !== runKey) {
    setAppliedKey(runKey);
    if (offset === 0) {
      setLoading(true);
      setError(null);
    } else {
      setLoadingMore(true);
    }
  }

  useEffect(() => {
    const runID = ++fetchRunRef.current;
    const stale = () => runID !== fetchRunRef.current;

    fetchFavorites({ limit: FAVORITES_PAGE_SIZE, offset })
      .then((res) => {
        if (stale()) return;
        setEntries((prev) => (offset === 0 ? res.entries : [...prev, ...res.entries]));
        setTotal(res.total);
      })
      .catch((e: unknown) => {
        if (!stale()) setError(errorMessage(e));
      })
      .finally(() => {
        if (stale()) return;
        setLoading(false);
        setLoadingMore(false);
      });
  }, [offset, attempt]);

  const loadMore = useCallback(() => {
    if (loading || loadingMore) return;
    setOffset(entries.length);
  }, [loading, loadingMore, entries.length]);

  const retryLoad = useCallback(() => {
    // Back to the first page: a retry replaces the list rather than appending.
    setOffset(0);
    setAttempt((n) => n + 1);
  }, []);

  return {
    entries,
    total,
    loading,
    loadingMore,
    error,
    retry: retryLoad,
    loadMore,
    hasMore: entries.length < total,
  };
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
  useClearOnChange(`${pinId}:${enabled}`, () => setSaved(false));

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