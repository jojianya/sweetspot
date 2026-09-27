"use client";

import { useEffect, useRef } from "react";
import { fetchSession } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { useAuth } from "@/store/auth";

/**
 * Reconciles the cached user with the real session credential, once per page
 * load.
 *
 * The session lives in an httpOnly cookie; localStorage only caches the user
 * object so the UI can render before the network answers. Those two can
 * disagree — a cleared, expired, or revoked cookie leaves a stale user in the
 * cache, and nothing else in the app would notice until the user happened to
 * open a page that calls a session-protected endpoint. The resulting 401 then
 * looks like clicking that page signed them out, when in truth the session had
 * already ended.
 *
 * Validating at startup turns that into an immediate, correct signed-out state
 * on the page the user is already looking at.
 */
export function useSessionSync() {
  const userId = useAuth((s) => s.user?.id);
  const clearAuth = useAuth((s) => s.clearAuth);
  // Once per page load, even if the effect re-runs on a re-render.
  const checked = useRef(false);

  useEffect(() => {
    if (!userId || checked.current) return;
    checked.current = true;
    let cancelled = false;

    fetchSession()
      .then(() => {
        // The session is live, so the cached user is trustworthy. Role
        // freshness on promotions and demotions is useSessionRefresh's job.
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        // 401 is the only outcome that says anything about the session: the
        // cookie is missing, expired, or revoked. The response interceptor has
        // already cleared the cached user, and clearing again is harmless.
        // Everything else — offline, timeout, 5xx — proves nothing about the
        // session, so the cached user is kept rather than signing the user out.
        if (error instanceof ApiError && error.status === 401) {
          clearAuth();
        }
      });

    return () => {
      cancelled = true;
    };
  }, [userId, clearAuth]);
}
