"use client";

import { useCallback, useEffect, useState } from "react";
import PanelSheet from "@/components/PanelSheet";
import { BookmarkIcon, CloseIcon } from "@/components/icons";
import { useFavorites } from "@/hooks/useFavorites";
import { useCollections } from "@/hooks/useCollections";
import { fetchCollection, removePinFromCollection } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { CollectionDetail, PinListEntry } from "@/lib/types";

interface SavedPinsPanelProps {
  onClose: () => void;
  onOpenPin: (entry: PinListEntry) => void;
  activeId: string | null;
}

type Tab = "saved" | "collections";

export default function SavedPinsPanel({
  onClose,
  onOpenPin,
  activeId,
}: SavedPinsPanelProps) {
  const [tab, setTab] = useState<Tab>("saved");
  const [openCollection, setOpenCollection] = useState<CollectionDetail | null>(null);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const [collectionError, setCollectionError] = useState<string | null>(null);

  const favorites = useFavorites();
  const collections = useCollections();
  const { entries, loading, error, retry } = favorites;
  const {
    collections: collectionList,
    loading: collectionsLoading,
    error: collectionsError,
    retry: collectionsRetry,
    create,
    removePin,
  } = collections;

  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);

  const title = (e: PinListEntry) =>
    e.caption?.trim() || (e.username ? `@${e.username}` : "Untitled");

  const subtitle = (e: PinListEntry) => (e.username ? `@${e.username}` : "Pin");

  const openCollectionDetail = useCallback(async (id: string) => {
    setOpeningId(id);
    setCollectionError(null);
    try {
      const detail = await fetchCollection(id);
      setOpenCollection(detail);
    } catch (e) {
      setCollectionError(errorMessage(e));
    } finally {
      setOpeningId(null);
    }
  }, []);

  const handleRemovePin = useCallback(
    async (collectionId: string, pinId: string) => {
      setOpenCollection((prev) => {
        if (!prev) return prev;
        const removed = prev.pins.filter((p) => p.id !== pinId);
        return { ...prev, pins: removed, pin_count: Math.max(0, prev.pin_count - 1) };
      });
      try {
        await removePin(collectionId, pinId);
      } catch (e) {
        setCollectionError(errorMessage(e));
        await openCollectionDetail(collectionId);
      }
    },
    [removePin, openCollectionDetail]
  );

  const handleCreate = async () => {
    const name = newName.trim();
    if (!name || creating) return;
    setCreating(true);
    setCollectionError(null);
    try {
      await create(name);
      setNewName("");
    } catch (e) {
      setCollectionError(errorMessage(e));
    } finally {
      setCreating(false);
    }
  };

  return (
    <PanelSheet role="dialog" aria-modal="true" aria-label="Saved pins" onClose={onClose}>
      <div className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-4 py-3.5 dark:border-zinc-800">
        <div className="flex items-center gap-2 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
          <span className="text-zinc-400">
            <BookmarkIcon className="h-4 w-4" />
          </span>
          {openCollection ? openCollection.name : tab === "saved" ? "Saved" : "Collections"}
          {!openCollection &&
            !loading &&
            tab === "saved" &&
            entries.length > 0 && (
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

      {/* Tabs */}
      {!openCollection && (
        <div className="flex shrink-0 gap-1 border-b border-zinc-100 px-3 py-2 dark:border-zinc-800">
          {(
            [
              { key: "saved", label: "Saved" },
              { key: "collections", label: "Collections" },
            ] as const
          ).map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              aria-pressed={tab === t.key}
              className={`flex-1 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                tab === t.key
                  ? "bg-zinc-100 text-zinc-900 dark:bg-zinc-800 dark:text-zinc-100"
                  : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-300"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-2 py-2">
        {openCollection ? (
          // Collection detail: pins inside plus a remove action per row.
          <div className="px-1">
            <div className="mb-2 flex items-center gap-2">
              <button
                type="button"
                onClick={() => setOpenCollection(null)}
                className="rounded-full p-1 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
                aria-label="Back to collections"
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
                  <path d="M19 12H5" />
                  <path d="m12 19-7-7 7-7" />
                </svg>
              </button>
              {openCollection.description && (
                <p className="truncate text-xs text-zinc-500 dark:text-zinc-400">
                  {openCollection.description}
                </p>
              )}
            </div>

            {collectionError && (
              <p className="mb-2 text-xs text-rose-600 dark:text-rose-400" role="alert">
                {collectionError}
              </p>
            )}

            {openCollection.pins.length === 0 && (
              <p className="px-3 py-8 text-center text-sm text-zinc-500 dark:text-zinc-400">
                No pins in this collection yet — open a pin and choose “Add to
                collection”.
              </p>
            )}

            <ul className="space-y-1">
              {openCollection.pins.map((entry) => {
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
                    <button
                      type="button"
                      onClick={() => handleRemovePin(openCollection.id, entry.id)}
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
        ) : tab === "saved" ? (
          <>
            {loading && (
              <p className="px-3 py-8 text-center text-sm text-zinc-500 dark:text-zinc-400">
                Loading saved pins…
              </p>
            )}

            {!loading && error && (
              <div className="px-3 py-8 text-center">
                <p className="text-sm text-rose-600 dark:text-rose-400">{error}</p>
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
          </>
        ) : (
          <>
            {/* Collections tab */}
            <div className="mb-3 flex gap-2 px-1">
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

            {collectionError && !openCollection && (
              <p className="mb-2 px-1 text-xs text-rose-600 dark:text-rose-400" role="alert">
                {collectionError}
              </p>
            )}

            {collectionsLoading && (
              <p className="px-3 py-8 text-center text-sm text-zinc-500 dark:text-zinc-400">
                Loading collections…
              </p>
            )}

            {!collectionsLoading && collectionsError && (
              <div className="px-3 py-8 text-center">
                <p className="text-sm text-rose-600 dark:text-rose-400">{collectionsError}</p>
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
                    {c.cover_url ? (
                      <img
                        src={c.cover_url}
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
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                        {c.name}
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
        )}
      </div>
    </PanelSheet>
  );
}