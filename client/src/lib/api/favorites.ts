import api from "./client";
import type { PinListEntry } from "@/lib/types";

export interface FavoriteEntry extends PinListEntry {
  saved_at: string;
}

export async function fetchFavoriteIDs(): Promise<string[]> {
  const { data } = await api.get<{ ids: string[] }>("/favorites/ids");
  return data.ids;
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