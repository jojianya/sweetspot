import api from "./client";
import { pinListEntrySchema } from "./schemas";
import type { PinListEntry, PublicProfile } from "@/lib/types";

/** Raw profile payload (GET /users/:id returns the user object directly). */
export async function fetchUser(userId: string): Promise<PublicProfile> {
  const { data } = await api.get<PublicProfile>(`/users/${userId}`);
  return data;
}

export async function fetchUserPins(userId: string, limit = 50): Promise<PinListEntry[]> {
  const { data } = await api.get<{ pins: unknown }>(`/users/${userId}/pins`, {
    params: { limit },
  });
  return pinListEntrySchema.array().parse(data.pins);
}