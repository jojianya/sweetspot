import api from "./client";
import type { PinListEntry } from "@/lib/types";

export interface FavoriteEntry extends PinListEntry {
  saved_at: string;
}

export async function fetchFavoriteIDs(): Promise<string[]> {
  const { data } = await api.get<{ ids: string[] }>("/favorites/ids");
  return data.ids;
}

/**
 * Whether one pin is saved by the current user, from GET /favorites/:id/saved.
 *
 * One indexed EXISTS probe per pin, so a detail view can learn the answer
 * without downloading every saved id (the /favorites/ids list is capped at
 * 5000 server-side precisely because it is meant to be a full membership set).
 */
export async function fetchIsSaved(pinId: string): Promise<boolean> {
  const { data } = await api.get<{ saved: boolean }>(`/favorites/${pinId}/saved`);
  return data.saved;
}

export async function saveFavorite(pinId: string): Promise<void> {
  await api.put(`/favorites/${pinId}`);
}

export async function removeFavorite(pinId: string): Promise<void> {
  await api.delete(`/favorites/${pinId}`);
}

/**
 * GET /favorites — one page of saved pins plus the total number of saved pins,
 * so the panel can tell a full list from a truncated one.
 */
export async function fetchFavorites(
  options: { limit?: number; offset?: number } = {}
): Promise<{ entries: FavoriteEntry[]; total: number }> {
  const { data } = await api.get<{ pins: FavoriteEntry[]; total?: number }>("/favorites", {
    params: options,
  });
  return { entries: data.pins, total: data.total ?? data.pins.length };
}