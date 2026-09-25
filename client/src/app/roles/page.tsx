"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import Avatar from "@/components/Avatar";
import { fetchUsers, searchUsers, updateUserRole } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import { useAuth } from "@/store/auth";
import type { PublicProfile } from "@/lib/types";

const ROLE_OPTIONS = ["user", "admin", "owner"] as const;

const ROLE_STYLES: Record<string, string> = {
  owner: "bg-rose-100 text-rose-700 dark:bg-rose-950/60 dark:text-rose-300",
  admin: "bg-sky-100 text-sky-700 dark:bg-sky-950/60 dark:text-sky-300",
  user: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400",
};

/** Users per page when listing all users. */
const PAGE_SIZE = 50;

export default function RolesPage() {
  const { token, user } = useAuth();
  const isOwner = user?.role === "owner";

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

  const fetchPage = useCallback((offset: number, append: boolean) => {
    fetchUsers({ limit: PAGE_SIZE, offset })
      .then((res) => {
        setTotal(res.total);
        setResults((prev) => (append ? [...prev, ...res.users] : res.users));
      })
      .catch((e: unknown) => setError(errorMessage(e)))
      .finally(() => {
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
    setLoading(true);
    setError(null);
    setNotice(null);
    try {
      const res = await searchUsers(q);
      setResults(res.users);
      setTotal(res.total);
      setActiveQuery(q);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const showAll = () => {
    setQuery("");
    setNotice(null);
    setError(null);
    setActiveQuery(null);
    setLoading(true);
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

  if (!token || !user) {
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-4 px-4 text-center">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Sign in to manage roles
          </h1>
          <Link
            href="/login"
            className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
          >
            Log in
          </Link>
        </div>
      </>
    );
  }

  if (!isOwner) {
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Owners only
          </h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            Only the account owner can change roles.
          </p>
          <Link
            href="/"
            className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
          >
            Back to map
          </Link>
        </div>
      </>
    );
  }

  return (
    <>
      <Navbar backHref="/reports" backLabel="Back to reports" />

      <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-xl font-bold text-zinc-900 dark:text-zinc-100">
            Manage roles
          </h1>
          <Link
            href="/reports"
            className="rounded-full border border-zinc-300 px-4 py-1.5 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Back to reports
          </Link>
        </div>

        <form onSubmit={handleSearch} className="mt-4 flex gap-2">
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search by username…"
            aria-label="Search users by username"
            className="min-w-0 flex-1 rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
          />
          <button
            type="submit"
            disabled={loading || !query.trim()}
            className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300"
          >
            {loading ? "Searching…" : "Search"}
          </button>
        </form>

        <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm text-zinc-500 dark:text-zinc-400">
          <span>
            {total} user{total === 1 ? "" : "s"}
            {total > 0 && isSearching && activeQuery && (
              <span className="text-zinc-400 dark:text-zinc-500">
                {" "}
                · showing {results.length} of {total}
              </span>
            )}
          </span>
          {isSearching && (
            <button
              type="button"
              onClick={showAll}
              className="rounded-full border border-zinc-300 px-3.5 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Show all users
            </button>
          )}
        </div>

        {error && (
          <p className="mt-3 text-sm text-rose-600 dark:text-rose-400" role="alert">
            {error}
          </p>
        )}

        {notice && (
          <p className="mt-3 text-sm text-emerald-600 dark:text-emerald-400" role="status">
            {notice}
          </p>
        )}

        {loading && (
          <p className="py-10 text-center text-sm text-zinc-400 dark:text-zinc-500">
            Loading users…
          </p>
        )}

        {!loading && error && (
          <div className="flex flex-col items-center gap-3 py-10 text-center">
            <p className="text-sm text-rose-600 dark:text-rose-400">
              Failed to load users.
            </p>
            <button
              type="button"
              onClick={retryLoad}
              className="rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Retry
            </button>
          </div>
        )}

        {!loading && !error && results.length === 0 && (
          <div className="mt-6 rounded-xl border border-dashed border-zinc-300 py-12 px-6 text-center dark:border-zinc-700">
            <p className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
              {isSearching
                ? `No users match "${activeQuery}"`
                : total === 0
                  ? "No users registered yet"
                  : "Nothing to show"}
            </p>
            {isSearching && (
              <button
                type="button"
                onClick={showAll}
                className="mt-3 rounded-full bg-rose-600 px-4 py-2 text-sm font-medium text-white hover:bg-rose-700"
              >
                Show all users
              </button>
            )}
          </div>
        )}

        {!loading && !error && results.length > 0 && (
          <>
            <ul className="mt-4 space-y-2.5">
              {results.map((profile) => {
                const isSelf = profile.id === user.id;
                return (
                  <li
                    key={profile.id}
                    className="rounded-xl border border-zinc-200/70 p-4 dark:border-zinc-800"
                  >
                    <div className="flex flex-wrap items-center gap-3">
                      <Avatar
                        src={profile.avatar_url}
                        username={profile.username}
                        className="h-9 w-9"
                        fallbackClassName="bg-rose-100 text-rose-700 dark:bg-rose-950/60 dark:text-rose-300"
                      />
                      <div className="min-w-0 flex-1">
                        <p className="flex items-center gap-1.5 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                          <span className="truncate">@{profile.username}</span>
                          {isSelf && (
                            <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                              you
                            </span>
                          )}
                        </p>
                        <span
                          className={`mt-0.5 inline-block rounded-full px-2 py-0.5 text-xs font-semibold ${ROLE_STYLES[profile.role] ?? ROLE_STYLES.user}`}
                        >
                          {profile.role}
                        </span>
                      </div>

                      {isSelf ? (
                        <p className="text-xs text-zinc-400 dark:text-zinc-500">
                          You can&apos;t change your own role.
                        </p>
                      ) : (
                        <div className="flex items-center gap-2">
                          <label className="sr-only" htmlFor={`role-${profile.id}`}>
                            Role for @{profile.username}
                          </label>
                          <select
                            id={`role-${profile.id}`}
                            value={profile.role}
                            onChange={(e) => void handleRoleChange(profile, e.target.value)}
                            disabled={busyId !== null}
                            className="rounded-lg border border-zinc-300 px-2.5 py-1.5 text-sm font-medium text-zinc-800 disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200"
                          >
                            {ROLE_OPTIONS.map((role) => (
                              <option key={role} value={role}>
                                {role}
                              </option>
                            ))}
                          </select>
                        </div>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>

            {hasMore && (
              <div className="mt-4 flex justify-center">
                <button
                  type="button"
                  onClick={loadMore}
                  disabled={loadingMore}
                  className="rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                >
                  {loadingMore ? "Loading…" : `Load more (${results.length} of ${total})`}
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </>
  );
}