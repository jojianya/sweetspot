"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { fetchUsers, searchUsers, updateUserRole } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { PublicProfile } from "@/lib/types";

/** Users per page when listing all users. */
export const USERS_PAGE_SIZE = 50;

/**
 * Paging, search and role mutation for the roles admin screen. Search and
 * the full list share one result set: a search replaces it, "show all"
 * restores it. Extracted unchanged from app/roles/page.tsx.
 */
export function useUsersAdmin() {
  const [query, setQuery] = useState("");
  /** null = showing the full user list; a string = showing search matches. */
  const [activeQuery, setActiveQuery] = useState<string | null>(null);
  const [results, setResults] = useState<PublicProfile[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const isSearching = activeQuery !== null;
  const hasMore = !isSearching && results.length > 0 && results.length < total;

  // Tag each run so a superseded page can neither overwrite a newer result set
  // nor clear the newer request's loading flags. Without this a "load more"
  // that is still in flight can land after a search and replace the matches
  // with the full list.
  const fetchRunRef = useRef(0);

  const fetchPage = useCallback((offset: number, append: boolean) => {
    const runID = ++fetchRunRef.current;
    const stale = () => runID !== fetchRunRef.current;
    fetchUsers({ limit: USERS_PAGE_SIZE, offset })
      .then((res) => {
        if (stale()) return;
        setTotal(res.total);
        setResults((prev) => (append ? [...prev, ...res.users] : res.users));
      })
      .catch((e: unknown) => {
        if (stale()) return;
        setError(errorMessage(e));
      })
      .finally(() => {
        if (stale()) return;
        setLoading(false);
        setLoadingMore(false);
      });
  }, []);

  useEffect(() => {
    fetchPage(0, false);
  }, [attempt, fetchPage]);

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    const q = query.trim();
    if (!q || loading || loadingMore) return;
    // Claim the run tag: any page still in flight is now stale, and a later
    // show-all (which bumps the tag via fetchPage) makes this search stale.
    const runID = ++fetchRunRef.current;
    const stale = () => runID !== fetchRunRef.current;
    setLoading(true);
    setError(null);
    setNotice(null);
    try {
      const res = await searchUsers(q);
      if (stale()) return;
      setResults(res.users);
      setTotal(res.total);
      setActiveQuery(q);
    } catch (err) {
      if (stale()) return;
      setError(errorMessage(err));
    } finally {
      if (stale()) return;
      setLoading(false);
    }
  };

  const showAll = () => {
    setQuery("");
    setNotice(null);
    setError(null);
    setActiveQuery(null);
    setLoading(true);
    // fetchPage bumps the tag itself, superseding any page in flight.
    fetchPage(0, false);
  };

  const loadMore = () => {
    if (loading || loadingMore) return;
    setLoadingMore(true);
    fetchPage(results.length, true);
  };

  const retryLoad = () => {
    setError(null);
    setLoading(true);
    setAttempt((n) => n + 1);
  };

  const handleRoleChange = async (profile: PublicProfile, role: string) => {
    if (busyId) return;
    setBusyId(profile.id);
    setError(null);
    setNotice(null);
    try {
      const updated = await updateUserRole(profile.id, role);
      setResults((prev) => prev.map((p) => (p.id === updated.id ? updated : p)));
      setNotice(`@${profile.username} is now ${role}.`);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusyId(null);
    }
  };

  return {
    query,
    setQuery,
    activeQuery,
    results,
    total,
    loading,
    loadingMore,
    error,
    notice,
    busyId,
    isSearching,
    hasMore,
    handleSearch,
    showAll,
    loadMore,
    retryLoad,
    handleRoleChange,
  };
}
