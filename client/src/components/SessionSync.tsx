"use client";

import { useEffect } from "react";
import { setUnauthorizedHandler } from "@/lib/api/client";
import { useAuth } from "@/store/auth";
import { useSessionSync } from "@/hooks/useSessionSync";

function clearSession() {
  useAuth.getState().clearAuth();
}

/**
 * Confines the client boundary for session reconciliation to one leaf
 * component instead of making the whole root layout a client component.
 * Renders nothing.
 */
export default function SessionSync() {
  // Registers the 401 backstop before the session check below. Effects never
  // run during SSR, so the server keeps the no-op default; assignment
  // overwrites, so StrictMode remounts and hot reloads stay correct.
  // Cleanup restores the no-op: unmount means leaving the app shell, where
  // no 401 handling is needed anymore.
  useEffect(() => {
    setUnauthorizedHandler(clearSession);
    return () => setUnauthorizedHandler(() => {});
  }, []);
  useSessionSync();
  return null;
}
