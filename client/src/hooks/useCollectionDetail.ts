"use client";

import { useCallback, useState } from "react";
import { fetchCollection, updateCollection } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import { useCollections } from "@/hooks/useCollections";
import type { CollectionDetail } from "@/lib/types";

/**
 * Open collection detail with its pin-removal and privacy mutations.
 * Removing a pin updates the open detail optimistically (pin dropped,
 * `pin_count - 1` floored at 0) and refetches on failure to roll back.
 * Extracted unchanged from SavedPinsPanel.
 */
export function useCollectionDetail() {
  const { removePin, patchLocal } = useCollections();
  const [openCollection, setOpenCollection] = useState<CollectionDetail | null>(null);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const [collectionError, setCollectionError] = useState<string | null>(null);
  const [togglingPrivate, setTogglingPrivate] = useState(false);

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
        // The rollback refetch starts by clearing the error; restore the
        // removal failure afterwards so it stays visible until the next
        // successful action or an explicit dismiss.
        const message = errorMessage(e);
        await openCollectionDetail(collectionId);
        setCollectionError(message);
      }
    },
    [removePin, openCollectionDetail]
  );

  const handleTogglePrivate = useCallback(
    async (isPrivate: boolean) => {
      if (!openCollection || togglingPrivate) return;
      setTogglingPrivate(true);
      setCollectionError(null);
      try {
        await updateCollection(openCollection.id, {
          name: openCollection.name,
          description: openCollection.description,
          isPrivate,
        });
        setOpenCollection({ ...openCollection, is_private: isPrivate });
        patchLocal(openCollection.id, { is_private: isPrivate });
      } catch (e) {
        setCollectionError(errorMessage(e));
      } finally {
        setTogglingPrivate(false);
      }
    },
    [openCollection, togglingPrivate, patchLocal]
  );

  return {
    openCollection,
    setOpenCollection,
    openingId,
    collectionError,
    setCollectionError,
    togglingPrivate,
    openCollectionDetail,
    handleRemovePin,
    handleTogglePrivate,
  };
}
