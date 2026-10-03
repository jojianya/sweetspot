"use client";

import { SkeletonRegion } from "./Skeleton";

/**
 * Shared page states for feed, reports and profile. Loading bodies, empty
 * copy and (currently) nothing else differ per page, so those stay with the
 * callers: PageLoading takes the skeleton as children, PageError is fully
 * shared (all three pages render it byte-identically), and PageEmpty takes
 * the title/description as props. No roles were added: the error text has
 * no alert role today and stays that way.
 */
export function PageLoading({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return <SkeletonRegion label={label}>{children}</SkeletonRegion>;
}

export function PageError({ error, onRetry }: { error: string; onRetry: () => void }) {
  return (
    <div className="flex flex-col items-center gap-3 py-10 text-center">
      <p className="text-sm text-rose-600 dark:text-rose-400">{error}</p>
      <button
        type="button"
        onClick={onRetry}
        className="rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
      >
        Retry
      </button>
    </div>
  );
}

export function PageEmpty({
  title,
  description,
}: {
  title: string;
  description?: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-dashed border-zinc-300 py-12 px-6 text-center dark:border-zinc-700">
      <p className="text-sm font-medium text-zinc-700 dark:text-zinc-300">{title}</p>
      {description}
    </div>
  );
}
