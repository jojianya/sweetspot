"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { errorMessage } from "@/lib/utils";

export interface UseAsyncDataOptions {
  /** When false the fetcher is not called (idle). Default true. */
  enabled?: boolean;
  /** Invoked with the user-facing message when a fetch fails. */
  onError?: (message: string) => void;
}

/**
 * Shared fetch-state hook: loads data on mount and whenever `deps` change,
 * exposing `loading`, `error`, and a `retry`. Each run gets an AbortSignal so
 * callers can cancel in-flight requests; results and errors from superseded
 * runs are dropped. Pass `enabled: false` to keep the hook idle, and use
 * `setData` for optimistic local updates.
 */
export function useAsyncData<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
  options: UseAsyncDataOptions = {}
) {
  const { enabled = true, onError } = options;
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(enabled);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const onErrorRef = useRef(onError);
  useEffect(() => {
    onErrorRef.current = onError;
  }, [onError]);

  // A "run" is identified by the caller's deps plus enabled/attempt. When it
  // changes, reset loading/error during render (the same derived-state pattern
  // the pin pages use) so a stale error never survives into a fresh fetch and
  // loading flips on without a setState-in-effect warning.
  const runKey = [...deps, enabled, attempt];
  const [prevRun, setPrevRun] = useState(runKey);
  if (
    enabled &&
    (prevRun.length !== runKey.length ||
      prevRun.some((value, i) => !Object.is(value, runKey[i])))
  ) {
    setPrevRun(runKey);
    setLoading(true);
    setError(null);
  }

  useEffect(() => {
    if (!enabled) return;

    const ctrl = new AbortController();
    fetcher(ctrl.signal)
      .then((result) => {
        if (!ctrl.signal.aborted) setData(result);
      })
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) {
          const message = errorMessage(e);
          setError(message);
          onErrorRef.current?.(message);
        }
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setLoading(false);
      });

    return () => ctrl.abort();
    // runKey is spread-compatible: the effect must run when any element
    // changes (element-wise comparison), which the spread in runKey encodes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runKey]);

  const retry = useCallback(() => {
    setError(null);
    setAttempt((n) => n + 1);
  }, []);

  return { data, loading, error, retry, setData };
}