"use client";

import { useState } from "react";
import { fetchSession } from "@/lib/api/auth";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useAuth } from "@/store/auth";

/**
 * Authorizes an admin surface against the server before rendering it.
 *
 * The persisted user in localStorage is a cache, not a credential: anyone can
 * edit it, and it survives a revoked session. Trusting it means the admin shell
 * renders for a caller who is not privileged, and its data fetches go out to
 * answer 403. Asking /me first makes the server's answer — the same one that
 * authorizes the endpoints — the one that decides whether the shell renders,
 * and it removes the pointless 401/403 round trips for signed-out or
 * unprivileged visitors.
 *
 * `authorized` is false until the check resolves and stays false if it fails,
 * so the admin UI is never rendered (and no admin request is issued) on the
 * strength of the cached role alone. The caller gates its data-fetching subtree
 * on `authorized`, which is why the fetch only happens for a confirmed caller.
 *
 * `fetchSession` needs no signal: a superseded /me answer is never applied,
 * because the run that is no longer current is dropped by useAsyncData.
 */
export function useAdminSession(required: "owner" | "moderator") {
  const { user } = useAuth();

  const { data, error } = useAsyncData(
    (signal) =>
      fetchSession().then((session) => {
        if (signal.aborted) throw new Error("aborted");
        return session;
      }),
    // A cached user is what makes the check worth making; with none there is
    // nothing to verify and the page shows its signed-out state.
    [user?.id ?? "", required],
    { enabled: user !== null }
  );

  // No cached user: nothing was verified, so nothing is authorized.
  if (user === null) return { authorized: false, checked: true };

  // A failed check proves nothing about the role, so it is not authorized.
  if (error !== null || data === null) return { authorized: false, checked: data !== null || error !== null };

  const role = data.role;
  const authorized = required === "owner" ? role === "owner" : role === "admin" || role === "owner";
  return { authorized, checked: true };
}