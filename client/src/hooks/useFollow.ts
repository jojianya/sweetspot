"use client";

import { useCallback, useEffect, useState } from "react";
import { fetchUserStats, followUser, unfollowUser } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { UserStats } from "@/lib/types";

/**
 * Follow/unfollow state for a profile. `enabled` gates the stats fetch (only
 * meaningful when viewing a real user). Toggle applies an optimistic update
 * and rolls back on failure.
 */
export function useFollow(userId: string, enabled: boolean) {
  const [stats, setStats] = useState<UserStats | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    fetchUserStats(userId)
      .then((data) => {
        if (!cancelled) setStats(data);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e));
      });
    return () => {
      cancelled = true;
    };
  }, [userId, enabled]);

  const toggle = useCallback(async () => {
    if (!stats || busy) return;
    setBusy(true);
    setError(null);
    const next = !stats.is_following;
    const rollback = stats;
    setStats({
      ...stats,
      is_following: next,
      followers: Math.max(0, stats.followers + (next ? 1 : -1)),
    });
    try {
      if (next) await followUser(userId);
      else await unfollowUser(userId);
    } catch (e) {
      setStats(rollback);
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }, [stats, busy, userId]);

  return { stats, busy, error, toggle };
}