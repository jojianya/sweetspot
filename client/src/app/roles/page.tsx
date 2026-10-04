"use client";

import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import Avatar from "@/components/Avatar";
import { useAdminSession } from "@/hooks/useAdminSession";
import { useUsersAdmin } from "@/hooks/useUsersAdmin";
import { useAuth } from "@/store/auth";

const ROLE_OPTIONS = ["user", "admin", "owner"] as const;

const ROLE_STYLES: Record<string, string> = {
  owner: "bg-rose-100 text-rose-700 dark:bg-rose-950/60 dark:text-rose-300",
  admin: "bg-sky-100 text-sky-700 dark:bg-sky-950/60 dark:text-sky-300",
  user: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400",
};

export default function RolesPage() {
  const { user } = useAuth();
  // The cached role in localStorage is not a credential, so /me decides
  // whether the admin shell renders at all. Until it answers, authorized is
  // false and the admin tree below is never mounted, so no admin request is
  // made on the strength of a cache the caller could have edited.
  const { authorized, checked } = useAdminSession("owner");

  if (!user) {
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

  if (!authorized) {
    // While the check is in flight there is nothing to conclude yet, so show
    // the same gate rather than flashing the admin shell.
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Owners only
          </h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            {checked
              ? "Only the account owner can change roles."
              : "Checking your access…"}
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

  return <RolesAdminShell />;
}

/**
 * The role console. Split out so useUsersAdmin — which fetches the user list on
 * mount — only ever runs for a caller /me has confirmed is an owner.
 */
function RolesAdminShell() {
  const { user } = useAuth();
  const {
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
  } = useUsersAdmin();

  return (
    <>
      <Navbar backHref="/" backLabel="Back to map" hideAccountNav />

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
                const isSelf = user !== null && profile.id === user.id;
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