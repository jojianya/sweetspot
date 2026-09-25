import api from "./client";
import { pinListEntrySchema, privateUserSchema, publicUserSchema } from "./schemas";
import type { PinListEntry, PublicProfile, User } from "@/lib/types";

/** Raw profile payload (GET /users/:id returns the user object directly). */
export async function fetchUser(userId: string): Promise<PublicProfile> {
  const { data } = await api.get<PublicProfile>(`/users/${userId}`);
  return data;
}

/**
 * GET /users/:id as the caller themselves — returns the full private profile
 * (includes email and updated_at). Used to refresh the session cache.
 */
export async function fetchMe(userId: string): Promise<User> {
  const { data } = await api.get<unknown>(`/users/${userId}`);
  return privateUserSchema.parse(data);
}

export async function fetchUserPins(userId: string, limit = 50): Promise<PinListEntry[]> {
  const { data } = await api.get<{ pins: unknown }>(`/users/${userId}/pins`, {
    params: { limit },
  });
  return pinListEntrySchema.array().parse(data.pins);
}

/** A page of users plus the total number of registered users. */
export interface UserListResult {
  users: PublicProfile[];
  total: number;
}

function parseUserList(data: { users: unknown; total?: unknown }): UserListResult {
  return {
    users: publicUserSchema.array().parse(data.users),
    total: typeof data.total === "number" ? data.total : 0,
  };
}

/**
 * GET /users — owner-only user listing for role management. With `q`, filters
 * to usernames containing the query; without, returns all users (paginated).
 * Always includes the total number of registered users.
 */
export async function fetchUsers(options: {
  q?: string;
  limit?: number;
  offset?: number;
} = {}): Promise<UserListResult> {
  const { data } = await api.get<{ users: unknown; total?: unknown }>("/users", {
    params: options,
  });
  return parseUserList(data);
}

/** GET /users?q= — convenience alias for filtering by username. */
export async function searchUsers(query: string, limit = 20): Promise<UserListResult> {
  return fetchUsers({ q: query, limit });
}

/** PATCH /users/:id/role — promote/demote a user's role, owner-only. */
export async function updateUserRole(userId: string, role: string): Promise<PublicProfile> {
  const { data } = await api.patch<unknown>(`/users/${userId}/role`, { role });
  return publicUserSchema.parse(data);
}

export interface ProfileEdit {
  username?: string;
  socials?: Record<string, unknown>;
  avatar?: File;
}

/**
 * PATCH /users/me — update the caller's own profile (multipart). Any subset of
 * username / socials / avatar may be sent; the server rejects an empty payload.
 * Returns the private profile (includes email).
 */
export async function updateMyProfile(edit: ProfileEdit): Promise<PublicProfile> {
  const form = new FormData();
  if (edit.username !== undefined) form.append("username", edit.username);
  if (edit.socials !== undefined) form.append("socials", JSON.stringify(edit.socials));
  if (edit.avatar) form.append("avatar", edit.avatar);
  const { data } = await api.patch<unknown>("/users/me", form);
  return publicUserSchema.parse(data);
}