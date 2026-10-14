"use client";

import { fetchTrendingPins } from "@/features/pins/api/pins";
import { useAsyncData } from "@/shared/hooks/useAsyncData";
import type { TrendingPin } from "@/shared/types/pin";

/**
 * Loads the pins trending in a viewport. Pass `null` to keep it idle (e.g.
 * while the trending panel is closed); each bbox change re-fetches.
 */
export function useTrending(bbox: string | null) {
  const { data, loading, error } = useAsyncData<TrendingPin[]>(
    (signal) => fetchTrendingPins(bbox as string, signal), // enabled guarantees bbox is non-null
    [bbox],
    { enabled: !!bbox }
  );

  return { pins: data ?? [], loading, error };
}