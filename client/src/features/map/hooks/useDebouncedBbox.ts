"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/**
 * Map bbox with debounced writes, so a burst of moveend events doesn't close
 * and reopen the SSE connection on every pan/zoom. The caller quantizes the
 * bbox to ~11 m so a string-identical value doesn't re-run consumers either
 * way. Pending updates are dropped on unmount.
 */
export function useDebouncedBbox(delayMs = 250) {
  const [bbox, setBbox] = useState<string | null>(null);
  const boundsTimerRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (boundsTimerRef.current !== null) {
        window.clearTimeout(boundsTimerRef.current);
      }
    };
  }, []);

  const updateBbox = useCallback(
    (bboxValue: string) => {
      if (boundsTimerRef.current !== null) {
        window.clearTimeout(boundsTimerRef.current);
      }
      boundsTimerRef.current = window.setTimeout(() => {
        setBbox(bboxValue);
        boundsTimerRef.current = null;
      }, delayMs);
    },
    [delayMs]
  );

  return { bbox, updateBbox };
}
