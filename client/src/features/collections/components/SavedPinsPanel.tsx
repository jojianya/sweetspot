"use client";

import { useState } from "react";
import PanelSheet from "@/shared/ui/PanelSheet";
import { BookmarkIcon, CloseIcon } from "@/shared/ui/icons";
import { CollectionDetail } from "./CollectionDetail";
import { CollectionsList } from "./CollectionsList";
import { SavedPinsList } from "./SavedPinsList";
import { useFavorites } from "../hooks/useFavorites";
import { useCollections } from "../hooks/useCollections";
import { useCollectionDetail } from "../hooks/useCollectionDetail";
import { errorMessage } from "@/shared/utils";
import { useAuth } from "@/features/auth/store/auth";
import type { PinListEntry } from "@/shared/types/pin";

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
  const {
    openCollection,
    setOpenCollection,
    openingId,
    collectionError,
    setCollectionError,
    togglingPrivate,
    openCollectionDetail,
    handleRemovePin,
    handleTogglePrivate,
  } = useCollectionDetail();

  const favorites = useFavorites();
  const collections = useCollections();
  const { entries, total, loading, loadingMore, error, retry, loadMore, hasMore } = favorites;
  const {
    collections: collectionList,
    loading: collectionsLoading,
    error: collectionsError,
    retry: collectionsRetry,
    create,
  } = collections;

  const [newName, setNewName] = useState("");
  const [newPrivate, setNewPrivate] = useState(false);
  const [creating, setCreating] = useState(false);
  const { user } = useAuth();

  const title = (e: PinListEntry) =>
    e.caption?.trim() || (e.username ? `@${e.username}` : "Untitled");

  const subtitle = (e: PinListEntry) => (e.username ? `@${e.username}` : "Pin");

  // The server's own count, not the loaded page's length: `total ?? entries.length`
  // only matters for a caller that stubs the hook without it.
  const savedCount = total ?? entries.length;

  const handleCreate = async () => {
    const name = newName.trim();
    if (!name || creating) return;
    setCreating(true);
    setCollectionError(null);
    try {
      await create(name, null, newPrivate);
      setNewName("");
      setNewPrivate(false);
    } catch (e) {
      setCollectionError(errorMessage(e));
    } finally {
      setCreating(false);
    }
  };

  const ownDetail = openCollection !== null && user !== null && openCollection.user_id === user.id;

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
                {savedCount}
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
          <CollectionDetail
            collection={openCollection}
            activeId={activeId}
            onOpenPin={onOpenPin}
            onBack={() => setOpenCollection(null)}
            ownDetail={ownDetail}
            togglingPrivate={togglingPrivate}
            onTogglePrivate={handleTogglePrivate}
            collectionError={collectionError}
            onDismissError={() => setCollectionError(null)}
            onRemovePin={(pinId) => handleRemovePin(openCollection.id, pinId)}
            title={title}
            subtitle={subtitle}
          />
        ) : tab === "saved" ? (
          <SavedPinsList
            entries={entries}
            savedCount={savedCount}
            loading={loading}
            error={error}
            retry={retry}
            loadMore={loadMore}
            hasMore={hasMore}
            loadingMore={loadingMore}
            activeId={activeId}
            onOpenPin={onOpenPin}
            title={title}
            subtitle={subtitle}
          />
        ) : (
          <CollectionsList
            collectionList={collectionList}
            collectionsLoading={collectionsLoading}
            collectionsError={collectionsError}
            collectionsRetry={collectionsRetry}
            openingId={openingId}
            openCollectionDetail={openCollectionDetail}
            onDismissError={() => setCollectionError(null)}
            newName={newName}
            setNewName={setNewName}
            newPrivate={newPrivate}
            setNewPrivate={setNewPrivate}
            creating={creating}
            handleCreate={handleCreate}
          />
        )}
      </div>
    </PanelSheet>
  );
}
