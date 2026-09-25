"use client";

import { useEffect, useRef, useState } from "react";
import { fetchTrendingPins } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { TrendingPin } from "@/lib/types";

/**
 * Loads the pins trending in a viewport. Pass `null` to keep it idle (e.g.
 * while the trending panel is closed); each bbox change re-fetches.
 */
export function useTrending(bbox: string | null) {
  const [pins, setPins] = useState<TrendingPin[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const [query, setQuery] = useState<{ bbox: string | null }>({ bbox });
  if (query.bbox !== bbox) {
    setQuery({ bbox });
    if (bbox) {
      setLoading(true);
      setError(null);
    }
  }

  useEffect(() => {
    if (!bbox) return;
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;

    fetchTrendingPins(bbox, ctrl.signal)
      .then((data) => {
        if (!ctrl.signal.aborted) setPins(data);
      })
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) setError(errorMessage(e));
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false);
      });

    return () => {
      ctrl.abort();
      if (abortRef.current === ctrl) abortRef.current = null;
    };
  }, [bbox]);

  return { pins, loading, error };
}