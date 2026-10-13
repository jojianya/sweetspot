"use client";

import { BackIcon, BookmarkIcon } from "@/components/icons";
import { PinCoverImage } from "@/components/pins/PinCoverImage";
import type { CollectionDetail, PinListEntry } from "@/lib/types";

/** Open-collection sub-screen: pins inside plus a remove action per row. */
export function CollectionDetail({
  collection,
  activeId,
  onOpenPin,
  onBack,
  ownDetail,
  togglingPrivate,
  onTogglePrivate,
  collectionError,
  onDismissError,
  onRemovePin,
  title,
  subtitle,
}: {
  collection: CollectionDetail;
  activeId: string | null;
  onOpenPin: (entry: PinListEntry) => void;
  onBack: () => void;
  ownDetail: boolean;
  togglingPrivate: boolean;
  onTogglePrivate: (isPrivate: boolean) => void;
  collectionError: string | null;
  onDismissError: () => void;
  onRemovePin: (pinId: string) => void;
  title: (e: PinListEntry) => string;
  subtitle: (e: PinListEntry) => string;
}) {
  return (
    <div className="px-1">
      <div className="mb-2 flex items-center gap-2">
        <button
          type="button"
          onClick={onBack}
          className="rounded-full p-1 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
          aria-label="Back to collections"
        >
          <BackIcon />
        </button>
        {collection.description && (
          <p className="truncate text-xs text-zinc-500 dark:text-zinc-400">
            {collection.description}
          </p>
        )}
      </div>
      {ownDetail && (
        <label className="mb-2 flex cursor-pointer items-center gap-2 px-1 text-xs text-zinc-500 dark:text-zinc-400">
          <input
            type="checkbox"
            checked={collection.is_private}
            disabled={togglingPrivate}
            onChange={(e) => void onTogglePrivate(e.target.checked)}
            className="h-3.5 w-3.5 rounded accent-rose-600 disabled:opacity-50"
          />
          Private — only you can see this collection
        </label>
      )}

      {collectionError && (
        <p className="mb-2 text-xs text-rose-600 dark:text-rose-400" role="alert">
          {collectionError}{" "}
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

      {collection.pins.length === 0 && (
        <p className="px-3 py-8 text-center text-sm text-zinc-500 dark:text-zinc-400">
          No pins in this collection yet — open a pin and choose “Add to
          collection”.
        </p>
      )}

      <ul className="space-y-1">
        {collection.pins.map((entry) => {
          const active = entry.id === activeId;
          return (
            <li
              key={entry.id}
              className={`flex items-center gap-3 rounded-xl px-2.5 py-2.5 transition-colors ${
                active
                  ? "bg-sky-50 ring-1 ring-inset ring-sky-200 dark:bg-sky-950/50 dark:ring-sky-800"
                  : "hover:bg-zinc-50 dark:hover:bg-zinc-800"
              }`}
            >
              <button
                type="button"
                onClick={() => onOpenPin(entry)}
                className="flex min-w-0 flex-1 items-center gap-3 text-left"
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
              <button
                type="button"
                onClick={() => onRemovePin(entry.id)}
                className="shrink-0 rounded-full p-1.5 text-zinc-300 transition-colors hover:bg-zinc-100 hover:text-rose-500 dark:text-zinc-600 dark:hover:bg-zinc-800 dark:hover:text-rose-400"
                aria-label={`Remove ${title(entry)} from collection`}
              >
                <svg
                  className="h-4 w-4"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth={2}
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden
                >
                  <path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" />
                </svg>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
