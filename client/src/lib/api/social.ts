import api from "./client";
import { pinListEntrySchema, userStatsSchema } from "./schemas";
import type { PinListEntry, UserStats } from "@/lib/types";

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