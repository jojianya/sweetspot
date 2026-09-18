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

export async function fetchFavorites(): Promise<FavoriteEntry[]> {
  const { data } = await api.get<{ pins: FavoriteEntry[] }>("/favorites");
  return data.pins;
}