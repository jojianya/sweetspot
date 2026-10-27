import api from "@/shared/api/client";
import { pinListEntrySchema, userStatsSchema } from "@/shared/api/schemas";
import type { PinListEntry } from "@/shared/types/pin"
import type { UserStats } from "@/shared/types/social";

export async function fetchUserStats(userId: string): Promise<UserStats> {
  const { data } = await api.get<unknown>(`/users/${userId}/stats`);
  return userStatsSchema.parse(data);
}

export async function followUser(userId: string): Promise<void> {
  await api.put(`/users/${userId}/follow`);
}

export async function unfollowUser(userId: string): Promise<void> {
  await api.delete(`/users/${userId}/follow`);
}

export async function fetchFeed(limit = 50): Promise<PinListEntry[]> {
  const { data } = await api.get<{ pins: unknown }>("/feed", { params: { limit } });
  return pinListEntrySchema.array().parse(data.pins);
}