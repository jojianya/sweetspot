import api from "./client";
import { collectionDetailSchema, collectionEntrySchema } from "./schemas";
import type { CollectionDetail, CollectionEntry } from "@/lib/types";

export async function fetchMyCollections(): Promise<CollectionEntry[]> {
  const { data } = await api.get<{ collections: unknown }>("/collections");
  return collectionEntrySchema.array().parse(data.collections);
}

export async function fetchUserCollections(userId: string): Promise<CollectionEntry[]> {
  const { data } = await api.get<{ collections: unknown }>(`/users/${userId}/collections`);
  return collectionEntrySchema.array().parse(data.collections);
}

export async function fetchCollection(id: string): Promise<CollectionDetail> {
  const { data } = await api.get<{ collection: unknown }>(`/collections/${id}`);
  return collectionDetailSchema.parse(data.collection);
}

export async function createCollection(
  name: string,
  description?: string | null
): Promise<CollectionEntry> {
  const { data } = await api.post<{ collection: unknown }>("/collections", {
    name,
    description: description || null,
  });
  return collectionEntrySchema.parse(data.collection);
}

export async function updateCollection(
  id: string,
  fields: { name: string; description?: string | null }
): Promise<void> {
  await api.patch(`/collections/${id}`, { ...fields, description: fields.description || null });
}

export async function deleteCollection(id: string): Promise<void> {
  await api.delete(`/collections/${id}`);
}

export async function addPinToCollection(collectionId: string, pinId: string): Promise<void> {
  await api.put(`/collections/${collectionId}/pins/${pinId}`);
}

export async function removePinFromCollection(collectionId: string, pinId: string): Promise<void> {
  await api.delete(`/collections/${collectionId}/pins/${pinId}`);
}