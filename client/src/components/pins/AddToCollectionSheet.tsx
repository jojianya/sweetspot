"use client";

import { useState } from "react";
import Link from "next/link";
import PanelSheet from "@/components/PanelSheet";
import { CloseIcon } from "@/components/icons";
import { useCollections } from "@/hooks/useCollections";
import { errorMessage } from "@/lib/utils";
import { useAuth } from "@/store/auth";

interface AddToCollectionSheetProps {
  pinId: string;
  onClose: () => void;
}

/** Picker used from pin detail: add the current pin to one of my collections. */
export default function AddToCollectionSheet({ pinId, onClose }: AddToCollectionSheetProps) {
  const { token } = useAuth();
  const { collections, loading, error, retry, create, addPin } = useCollections();
  const [name, setName] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [addedIds, setAddedIds] = useState<string[]>([]);

  if (!token) {
    return (
      <PanelSheet role="dialog" aria-modal="true" aria-label="Add to collection" onClose={onClose}>
        <header className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
          <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Add to collection
          </h2>
          <button type="button" onClick={onClose} aria-label="Close" className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800">
            <CloseIcon />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto p-5 text-sm text-zinc-600 dark:text-zinc-400">
          <p className="mb-3">Sign in to save pins into collections.</p>
          <Link
            href="/login"
            className="inline-block rounded-lg bg-rose-600 px-4 py-2 font-medium text-white hover:bg-rose-700"
          >
            Log in
          </Link>
        </div>
      </PanelSheet>
    );
  }

  const handleCreate = async () => {
    const trimmed = name.trim();
    if (!trimmed || busy) return;
    setBusy("new");
    setMessage(null);
    try {
      const collection = await create(trimmed);
      await addPin(collection.id, pinId);
      setAddedIds((prev) => [...prev, collection.id]);
      setName("");
      setMessage(`Created "${trimmed}" and added the pin.`);
    } catch (e) {
      setMessage(errorMessage(e));
    } finally {
      setBusy(null);
    }
  };

  const handleAdd = async (collectionId: string) => {
    if (busy) return;
    setBusy(collectionId);
    setMessage(null);
    try {
      await addPin(collectionId, pinId);
      setAddedIds((prev) => [...prev, collectionId]);
    } catch (e) {
      setMessage(errorMessage(e));
    } finally {
      setBusy(null);
    }
  };

  return (
    <PanelSheet role="dialog" aria-modal="true" aria-label="Add to collection" onClose={onClose}>
      <header className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
        <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
          Add to collection
        </h2>
        <button type="button" onClick={onClose} aria-label="Close" className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800">
          <CloseIcon />
        </button>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        <div className="flex gap-2">
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={60}
            placeholder="New collection name"
            className="min-w-0 flex-1 rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
            aria-label="New collection name"
          />
          <button
            type="button"
            onClick={handleCreate}
            disabled={!name.trim() || busy !== null}
            className="shrink-0 rounded-lg bg-rose-600 px-3.5 py-2 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-50"
          >
            {busy === "new" ? "…" : "Create"}
          </button>
        </div>

        {message && (
          <p className="mt-3 text-sm text-rose-600 dark:text-rose-400" role="status">
            {message}
          </p>
        )}

        <div className="mt-4">
          {loading && (
            <p className="py-4 text-center text-sm text-zinc-400 dark:text-zinc-500">
              Loading collections…
            </p>
          )}

          {!loading && error && (
            <div className="flex items-center gap-2 py-4 text-sm text-rose-600 dark:text-rose-400">
              <span>{error}</span>
              <button type="button" onClick={retry} className="font-semibold underline">
                Retry
              </button>
            </div>
          )}

          {!loading && !error && collections.length === 0 && (
            <p className="py-4 text-center text-sm text-zinc-400 dark:text-zinc-500">
              No collections yet — create one above.
            </p>
          )}

          <ul className="space-y-1.5">
            {collections.map((c) => {
              const added = addedIds.includes(c.id);
              return (
                <li key={c.id}>
                  <button
                    type="button"
                    onClick={() => handleAdd(c.id)}
                    disabled={added || busy === c.id}
                    className="flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left transition-colors hover:bg-zinc-50 disabled:opacity-60 dark:hover:bg-zinc-800"
                  >
                    {c.cover_url ? (
                      <img
                        src={c.cover_url}
                        alt=""
                        className="h-11 w-11 shrink-0 rounded-lg object-cover"
                      />
                    ) : (
                      <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600">
                        <CloseIcon className="h-4 w-4 rotate-45" />
                      </span>
                    )}
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                        {c.name}
                      </span>
                      <span className="block text-xs text-zinc-500 dark:text-zinc-400">
                        {c.pin_count} pin{c.pin_count === 1 ? "" : "s"}
                      </span>
                    </span>
                    <span className="shrink-0 text-xs font-medium text-sky-600 dark:text-sky-400">
                      {added ? "Added ✓" : busy === c.id ? "Adding…" : "Add"}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      </div>
    </PanelSheet>
  );
}