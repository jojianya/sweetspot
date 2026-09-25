"use client";

import { useCallback, useEffect, useState } from "react";
import { fetchUserStats, followUser, unfollowUser } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { UserStats } from "@/lib/types";

interface FollowState {
  userId: string;
  stats: UserStats | null;
  error: string | null;
}

/**
 * Loads stats for a profile and manages follow/unfollow state. Results are
 * tagged by user ID so a client-side route change cannot expose the previous
 * profile's controls while the new request is pending.
 */
export function useFollow(userId: string) {
  const [state, setState] = useState<FollowState | null>({ userId, stats: null, error: null });
  const [busyUserId, setBusyUserId] = useState<string | null>(null);

  const current = state?.userId === userId ? state : null;
  const stats = current?.stats ?? null;
  const error = current?.error ?? null;
  const busy = busyUserId === userId;

  useEffect(() => {
    let cancelled = false;
    fetchUserStats(userId)
      .then((data) => {
        if (!cancelled) setState({ userId, stats: data, error: null });
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setState({ userId, stats: null, error: errorMessage(e) });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [userId]);

  const toggle = useCallback(async () => {
    if (!stats || busy) return;
    setBusyUserId(userId);
    const rollback = stats;
    const next = !stats.is_following;
    setState((currentState) =>
      currentState?.userId === userId && currentState.stats
        ? {
            userId,
            stats: {
              ...currentState.stats,
              is_following: next,
              followers: Math.max(0, currentState.stats.followers + (next ? 1 : -1)),
            },
            error: null,
          }
        : currentState
    );
    try {
      if (next) await followUser(userId);
      else await unfollowUser(userId);
    } catch (e) {
      setState((currentState) =>
        currentState?.userId === userId
          ? { userId, stats: rollback, error: errorMessage(e) }
          : currentState
      );
    } finally {
      setBusyUserId((currentUserId) => (currentUserId === userId ? null : currentUserId));
    }
  }, [stats, busy, userId]);

  return { stats, busy, error, toggle };
}