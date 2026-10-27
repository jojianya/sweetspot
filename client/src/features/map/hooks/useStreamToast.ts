"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/**
 * Short-lived realtime counter: bumps on each streamed pin and auto-clears
 * after a few seconds. The timer is dropped on unmount. Callbacks are stable
 * so stream subscriptions never re-run because of the toast.
 */
export function useStreamToast(timeoutMs = 4000) {
  const [count, setCount] = useState(0);
  const timerRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
      }
    };
  }, []);

  const bump = useCallback(() => {
    setCount((n) => n + 1);
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    timerRef.current = window.setTimeout(() => setCount(0), timeoutMs);
  }, [timeoutMs]);

  const dismiss = useCallback(() => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    setCount(0);
  }, []);

  return { count, bump, dismiss };
}
