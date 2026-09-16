"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { fetchPins } from "@/lib/api";
import type { PinListEntry } from "@/lib/types";

export function usePins(bbox: string | null, category: number | null) {
  const [pins, setPins] = useState<PinListEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const [query, setQuery] = useState<{
    bbox: string | null;
    category: number | null;
  }>({ bbox, category });
  if (query.bbox !== bbox || query.category !== category) {
    setQuery({ bbox, category });
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

    fetchPins(bbox, category, ctrl.signal)
      .then((data) => {
        if (!ctrl.signal.aborted) setPins(data);
      })
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) {
          setError(e instanceof Error ? e.message : "something went wrong");
        }
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false);
      });

    return () => {
      ctrl.abort();
      if (abortRef.current === ctrl) abortRef.current = null;
    };
  }, [bbox, category]);

  const addPin = useCallback((pin: PinListEntry) => {
    setPins((prev) =>
      prev.some((p) => p.id === pin.id) ? prev : [pin, ...prev]
    );
  }, []);

  return { pins, loading, error, addPin };
}