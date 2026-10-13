"use client";

import { useState } from "react";

/**
 * Derived-state reset shared by the data hooks. When key changes between
 * renders, the stored key updates and clear() runs during render (not in an
 * effect), so the very first paint for the new identity already shows cleared
 * state with no one-frame flash of the old value. React re-renders
 * immediately on render-phase setState, which is what makes this legal.
 *
 * Keys are compared with Object.is; every caller passes strings, for which
 * that matches !==.
 */
export function useClearOnChange(key: unknown, clear: () => void): void {
  const [prev, setPrev] = useState(key);
  if (!Object.is(prev, key)) {
    setPrev(key);
    clear();
  }
}
