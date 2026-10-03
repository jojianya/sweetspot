"use client";

import { useEffect, useState } from "react";
import { fetchUser, fetchUserCollections, fetchUserPins } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import { errorMessage } from "@/lib/utils";
import type { CollectionEntry, PinListEntry, PublicProfile } from "@/lib/types";

/**
 * Profile data fan-out: loads the profile, its pins and its collections in
 * parallel. A 404 maps to `notFound`; anything else maps to `error`.
 * `retry` clears the failure and re-runs the load. Extracted unchanged from
 * app/users/[id]/page.tsx.
 */
export function useProfile(id: string) {
  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [pins, setPins] = useState<PinListEntry[]>([]);
  const [collections, setCollections] = useState<CollectionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [prof, userPins, userCollections] = await Promise.all([
          fetchUser(id),
          fetchUserPins(id),
          fetchUserCollections(id),
        ]);
        if (cancelled) return;
        setProfile(prof);
        setPins(userPins);
        setCollections(userCollections);
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(errorMessage(e));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [id, attempt]);

  const retry = () => {
    setError(null);
    setNotFound(false);
    setLoading(true);
    setAttempt((n) => n + 1);
  };

  return { profile, setProfile, pins, collections, loading, notFound, error, retry };
}
