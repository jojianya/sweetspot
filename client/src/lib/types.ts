export interface Category {
  id: number;
  name: string;
}

export interface PinPhoto {
  id: string;
  pin_id: string;
  photo_url: string;
  thumbnail_url: string;
  position: number;
  created_at: string;
}

export interface PinListEntry {
  id: string;
  user_id: string;
  location: string;
  geohash: string;
  caption: string | null;
  category_id: number;
  is_hidden: boolean;
  created_at: string;
  cover_url: string;
  username: string;
}

export interface PinDetail {
  id: string;
  user_id: string;
  location: string;
  geohash: string;
  caption: string | null;
  category_id: number;
  is_hidden: boolean;
  created_at: string;
  category: string | null;
  username: string;
  avatar_url: string | null;
  photos: PinPhoto[];
}

export interface NewPinPhoto {
  photo_url: string;
  thumbnail_url: string;
  position: number;
}

export interface User {
  id: string;
  email: string;
  username: string;
  avatar_url: string | null;
  socials: Record<string, unknown>;
  role: string;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  token: string;
}

export interface Bbox {
  minLat: number;
  minLng: number;
  maxLat: number;
  maxLng: number;
}