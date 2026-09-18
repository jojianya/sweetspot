"use client";

import { useEffect, useState } from "react";
import PanelSheet from "@/components/PanelSheet";
import { BookmarkIcon, CloseIcon } from "@/components/icons";
import { fetchFavorites, type FavoriteEntry } from "@/lib/api";
import { errorMessage } from "@/lib/utils";

interface SavedPinsPanelProps {
  onClose: () => void;
  onOpenPin: (entry: FavoriteEntry) => void;
  activeId: string | null;
}

export default function SavedPinsPanel({
  onClose,
  onOpenPin,
  activeId,
}: SavedPinsPanelProps) {
  const [entries, setEntries] = useState<FavoriteEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = (resetLoading: boolean) => {
    if (resetLoading) setLoading(true);
    setError(null);
    fetchFavorites()
      .then(setEntries)
      .catch((e: unknown) => setError(errorMessage(e)))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    let cancelled = false;
    fetchFavorites()
      .then((data) => {
        if (!cancelled) setEntries(data);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const title = (e: FavoriteEntry) =>
    e.caption?.trim() || (e.username ? `@${e.username}` : "Untitled");

  const subtitle = (e: FavoriteEntry) => (e.username ? `@${e.username}` : "Saved pin");

  return (
    <PanelSheet role="dialog" aria-modal="true" aria-label="Saved pins">
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-4 py-3.5 dark:border-zinc-800">
        <div className="flex items-center gap-2 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
          <span className="text-zinc-400">
            <BookmarkIcon className="h-4 w-4" />
          </span>
          Saved
          {!loading && entries.length > 0 && (
            <span className="text-xs font-medium text-zinc-400">
              {entries.length}
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={onClose}
          className="flex h-8 w-8 items-center justify-center rounded-full text-zinc-500 transition-colors hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
          aria-label="Close saved list"
        >
          <CloseIcon />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto px-2 py-2">
        {loading && (
          <p className="px-3 py-8 text-center text-sm text-zinc-500 dark:text-zinc-400">Loading saved pins…</p>
        )}

        {!loading && error && (
          <div className="px-3 py-8 text-center">
            <p className="text-sm text-rose-600 dark:text-rose-400">{error}</p>
            <button
              type="button"
              onClick={() => load(true)}
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
                {entry.cover_url ? (
                  <img
                    src={entry.cover_url}
                    alt=""
                    loading="lazy"
                    decoding="async"
                    className="h-14 w-14 shrink-0 rounded-xl object-cover"
                  />
                ) : (
                  <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600">
                    <BookmarkIcon className="h-4 w-4" />
                  </span>
                )}
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
      </div>
    </PanelSheet>
  );
}