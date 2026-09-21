import type { PinListEntry } from "./pin";

export interface Comment {
  id: string;
  pin_id: string;
  user_id: string;
  body: string;
  is_hidden: boolean;
  created_at: string;
  username: string | null;
  avatar_url: string | null;
}

export interface CollectionEntry {
  id: string;
  user_id: string;
  name: string;
  description: string | null;
  created_at: string;
  pin_count: number;
  cover_url: string;
}

export interface CollectionDetail extends CollectionEntry {
  pins: PinListEntry[];
}

export interface UserStats {
  followers: number;
  following: number;
  pins_count: number;
  is_following: boolean;
}

/** Public profile payload from GET /users/:id. */
export interface PublicProfile {
  id: string;
  username: string;
  avatar_url: string | null;
  socials: Record<string, unknown>;
  role: string;
  created_at: string;
  email?: string;
}