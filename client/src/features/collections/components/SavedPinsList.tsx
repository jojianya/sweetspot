"use client";

import { BookmarkIcon } from "@/shared/ui/icons";
import { ListSkeleton } from "@/features/pins/components/ListSkeleton";
import { PinCoverImage } from "@/features/pins/components/PinCoverImage";
import type { PinListEntry } from "@/shared/types/pin";
import type { FavoriteEntry } from "../api/favorites";

/** Saved tab: favorite pins with load-more paging. */
export function SavedPinsList({
  entries,
  savedCount,
  loading,
  error,
  retry,
  loadMore,
  hasMore,
  loadingMore,
  activeId,
  onOpenPin,
  title,
  subtitle,
}: {
  entries: FavoriteEntry[];
  savedCount: number;
  loading: boolean;
  error: string | null;
  retry: () => void;
  loadMore: () => void;
  hasMore: boolean;
  loadingMore: boolean;
  activeId: string | null;
  onOpenPin: (entry: PinListEntry) => void;
  title: (e: PinListEntry) => string;
  subtitle: (e: PinListEntry) => string;
}) {
  return (
    <>
      {loading && <ListSkeleton label="Loading saved pins…" />}

      {!loading && error && (
        <div className="px-3 py-8 text-center">
          <p className="text-sm text-rose-600 dark:text-rose-400" role="alert">{error}</p>
          <button
            type="button"
            onClick={retry}
            className="mt-3 rounded-full border border-zinc-200 px-4 py-1.5 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Retry
          </button>
        </div>
      )}

      {!loading && !error && entries.length === 0 && (
        <p className="px-3 py-8 text-center text-sm text-zinc-500 dark:text-zinc-400">
          No saved pins yet — tap the bookmark on any pin to save it.
        </p>
      )}

      {!loading &&
        !error &&
        entries.map((entry) => {
          const active = entry.id === activeId;
          return (
            <button
              key={entry.id}
              type="button"
              onClick={() => onOpenPin(entry)}
              aria-pressed={active}
              className={`flex w-full items-center gap-3 rounded-xl px-2.5 py-2.5 text-left transition-colors ${
                active
                  ? "bg-sky-50 ring-1 ring-inset ring-sky-200 dark:bg-sky-950/50 dark:ring-sky-800"
                  : "hover:bg-zinc-50 dark:hover:bg-zinc-800"
              }`}
            >
              <PinCoverImage
                url={entry.cover_url}
                boxClass="h-14 w-14 rounded-xl"
                glyph={<BookmarkIcon className="h-4 w-4" />}
              />
              <span className="min-w-0">
                <span className="block truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                  {title(entry)}
                </span>
                <span className="block truncate text-xs text-zinc-500 dark:text-zinc-400">
                  {subtitle(entry)}
                </span>
              </span>
            </button>
          );
        })}
        {hasMore && (
          <div className="mt-2 flex justify-center">
            <button
              type="button"
              onClick={loadMore}
              disabled={loadingMore}
              className="rounded-full border border-zinc-200 px-4 py-1.5 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              {loadingMore ? "Loading…" : `Load more (${entries.length} of ${savedCount})`}
            </button>
          </div>
        )}
    </>
  );
}
