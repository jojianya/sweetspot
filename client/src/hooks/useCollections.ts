"use client";

import { useCallback } from "react";
import {
  addPinToCollection,
  createCollection,
  fetchMyCollections,
  removePinFromCollection,
} from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { CollectionEntry } from "@/lib/types";

/**
 * The viewer's collections plus the pin-membership actions used by the
 * "add to collection" pickers and the saved panel's collections tab.
 * Create/add/remove all update local state immediately for a snappy UI.
 */
export function useCollections() {
  const { data, loading, error, retry, setData } = useAsyncData<CollectionEntry[]>(
    () => fetchMyCollections(),
    []
  );

  const create = useCallback(
    async (name: string, description?: string | null) => {
      const collection = await createCollection(name, description);
      setData((prev) => [collection, ...(prev ?? [])]);
      return collection;
    },
    [setData]
  );

  const addPin = useCallback(
    async (collectionId: string, pinId: string) => {
      await addPinToCollection(collectionId, pinId);
      setData((prev) =>
        (prev ?? []).map((c) =>
          c.id === collectionId ? { ...c, pin_count: c.pin_count + 1 } : c
        )
      );
    },
    [setData]
  );

  const removePin = useCallback(
    async (collectionId: string, pinId: string) => {
      await removePinFromCollection(collectionId, pinId);
      setData((prev) =>
        (prev ?? []).map((c) =>
          c.id === collectionId
            ? { ...c, pin_count: Math.max(0, c.pin_count - 1) }
            : c
        )
      );
    },
    [setData]
  );

  return { collections: data ?? [], loading, error, retry, create, addPin, removePin };
}