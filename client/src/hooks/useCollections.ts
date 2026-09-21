"use client";

import { useCallback, useEffect, useState } from "react";
import {
  addPinToCollection,
  createCollection,
  fetchMyCollections,
  removePinFromCollection,
} from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { CollectionEntry } from "@/lib/types";

/**
 * The viewer's collections plus the pin-membership actions used by the
 * "add to collection" pickers and the saved panel's collections tab.
 * Create/add/remove all update local state immediately for a snappy UI.
 */
export function useCollections() {
  const [collections, setCollections] = useState<CollectionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    fetchMyCollections()
      .then((data) => {
        if (!cancelled) setCollections(data);
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
  }, [attempt]);

  const retry = useCallback(() => {
    setError(null);
    setLoading(true);
    setAttempt((n) => n + 1);
  }, []);

  const create = useCallback(async (name: string, description?: string | null) => {
    const collection = await createCollection(name, description);
    setCollections((prev) => [collection, ...prev]);
    return collection;
  }, []);

  const addPin = useCallback(async (collectionId: string, pinId: string) => {
    await addPinToCollection(collectionId, pinId);
    setCollections((prev) =>
      prev.map((c) =>
        c.id === collectionId ? { ...c, pin_count: c.pin_count + 1 } : c
      )
    );
  }, []);

  const removePin = useCallback(async (collectionId: string, pinId: string) => {
    await removePinFromCollection(collectionId, pinId);
    setCollections((prev) =>
      prev.map((c) =>
        c.id === collectionId
          ? { ...c, pin_count: Math.max(0, c.pin_count - 1) }
          : c
      )
    );
  }, []);

  return { collections, loading, error, retry, create, addPin, removePin };
}