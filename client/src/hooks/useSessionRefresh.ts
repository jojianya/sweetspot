import { useEffect } from "react";
import { fetchMe } from "@/lib/api/users";
import { useAuth } from "@/store/auth";

/**
 * Re-sync the persisted session with the server on mount so role changes
 * (promotions/demotions) show up in the UI without re-logging in. The server
 * role gates already read the role from the DB per request; this keeps the
 * client cache in step with them.
 */
export function useSessionRefresh() {
  const { user, token, setAuth } = useAuth();
  const userId = user?.id;

  useEffect(() => {
    if (!token || !userId) return;
    let cancelled = false;
    fetchMe(userId)
      .then((fresh) => {
        if (!cancelled) setAuth(fresh, token);
      })
      .catch(() => {
        // keep the cached user if the refresh fails (offline etc.)
      });
    return () => {
      cancelled = true;
    };
  }, [token, userId, setAuth]);
}