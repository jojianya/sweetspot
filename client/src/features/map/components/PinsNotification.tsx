"use client";

import { Skeleton } from "@/shared/ui/Skeleton";

/** Map status chrome: pins loading skeleton or the aggregated error banner. */
export function PinsNotification({
  loading,
  error,
  showRetry,
  onRetry,
}: {
  loading: boolean;
  error: string | null;
  showRetry: boolean;
  onRetry: () => void;
}) {
  if (loading) {
    return (
      <div role="status" aria-busy="true" aria-label="Loading pins…">
        <span className="sr-only">Loading pins…</span>
        <div className="absolute left-3 top-36 z-10 rounded bg-zinc-100/90 px-3 py-2 text-xs font-medium text-zinc-500 shadow dark:bg-zinc-800/90 dark:text-zinc-400" aria-hidden="true">
          <Skeleton className="h-3 w-32" />
        </div>
      </div>
    );
  }
  if (error) {
    return (
      <div
        role="alert"
        className="absolute left-3 top-40 z-10 rounded bg-rose-50 px-2 py-1 text-xs text-rose-600 shadow dark:bg-rose-950/60 dark:text-rose-300"
      >
        {error}
        {showRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="ml-2 font-semibold underline"
          >
            Retry
          </button>
        )}
      </div>
    );
  }
  return null;
}
