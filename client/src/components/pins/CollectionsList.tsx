"use client";

import { BookmarkIcon } from "@/shared/ui/icons";
import { ListSkeleton } from "@/components/pins/ListSkeleton";
import { PinCoverImage } from "@/components/pins/PinCoverImage";
import type { CollectionEntry } from "@/shared/types/social";

/** Collections tab: create form plus the collection list. */
export function CollectionsList({
  collectionList,
  collectionsLoading,
  collectionsError,
  collectionsRetry,
  openingId,
  openCollectionDetail,
  onDismissError,
  newName,
  setNewName,
  newPrivate,
  setNewPrivate,
  creating,
  handleCreate,
}: {
  collectionList: CollectionEntry[];
  collectionsLoading: boolean;
  collectionsError: string | null;
  collectionsRetry: () => void;
  openingId: string | null;
  openCollectionDetail: (id: string) => void;
  onDismissError: () => void;
  newName: string;
  setNewName: (value: string) => void;
  newPrivate: boolean;
  setNewPrivate: (value: boolean) => void;
  creating: boolean;
  handleCreate: () => void;
}) {
  return (
    <>
      {/* Collections tab */}
      <div className="mb-2 flex gap-2 px-1">
        <input
          type="text"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          maxLength={60}
          placeholder="New collection name"
          className="min-w-0 flex-1 rounded-lg border border-zinc-300 px-3 py-1.5 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
          aria-label="New collection name"
        />
        <button
          type="button"
          onClick={handleCreate}
          disabled={!newName.trim() || creating}
          className="shrink-0 rounded-lg bg-rose-600 px-3.5 py-1.5 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-50"
        >
          {creating ? "…" : "Create"}
        </button>
      </div>
      <label className="mb-3 flex cursor-pointer items-center gap-2 px-1 text-xs text-zinc-500 dark:text-zinc-400">
        <input
          type="checkbox"
          checked={newPrivate}
          onChange={(e) => setNewPrivate(e.target.checked)}
          className="h-3.5 w-3.5 rounded accent-rose-600"
        />
        Private — only you can see it
      </label>

      {collectionsError && (
        <p className="mb-2 px-1 text-xs text-rose-600 dark:text-rose-400" role="alert">
          {collectionsError}{" "}
          <button
            type="button"
            onClick={onDismissError}
            className="font-medium underline"
            aria-label="Dismiss error"
          >
            Dismiss
          </button>
        </p>
      )}

      {collectionsLoading && <ListSkeleton label="Loading collections…" />}

      {!collectionsLoading && collectionsError && (
        <div className="px-3 py-8 text-center">
          <p className="text-sm text-rose-600 dark:text-rose-400" role="alert">{collectionsError}</p>
          <button
            type="button"
            onClick={collectionsRetry}
            className="mt-3 rounded-full border border-zinc-200 px-4 py-1.5 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Retry
          </button>
        </div>
      )}

      {!collectionsLoading && !collectionsError && collectionList.length === 0 && (
        <p className="px-3 py-8 text-center text-sm text-zinc-500 dark:text-zinc-400">
          No collections yet — create one above.
        </p>
      )}

      <ul className="space-y-1">
        {collectionList.map((c) => (
          <li key={c.id}>
            <button
              type="button"
              onClick={() => openCollectionDetail(c.id)}
              disabled={openingId === c.id}
              className="flex w-full items-center gap-3 rounded-xl px-2.5 py-2.5 text-left transition-colors hover:bg-zinc-50 disabled:opacity-60 dark:hover:bg-zinc-800"
            >
              <PinCoverImage
                url={c.cover_url}
                boxClass="h-14 w-14 rounded-xl"
                glyph={<BookmarkIcon className="h-4 w-4" />}
              />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                  {c.name}
                  {c.is_private && (
                    <span className="ml-1.5 rounded-full bg-zinc-100 px-1.5 py-0.5 text-[10px] font-medium text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                      Private
                    </span>
                  )}
                </span>
                <span className="block truncate text-xs text-zinc-500 dark:text-zinc-400">
                  {c.pin_count} pin{c.pin_count === 1 ? "" : "s"}
                  {openingId === c.id ? " · opening…" : ""}
                </span>
              </span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}
