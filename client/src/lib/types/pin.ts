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
  user_id: string | null;
  location: string;
  geohash: string;
  caption: string | null;
  category_id: number;
  is_hidden: boolean;
  views: number;
  /** "Good spot" reaction total, maintained server-side as a column. */
  good_spot_count: number;
  created_at: string;
  cover_url: string;
  username: string | null;
}

/** A viewport list entry plus the engagement metrics behind the trending rank. */
export interface TrendingPin extends PinListEntry {
  comment_count: number;
  score: number;
}

export interface PinDetail {
  id: string;
  user_id: string | null;
  location: string;
  geohash: string;
  caption: string | null;
  category_id: number;
  is_hidden: boolean;
  views: number;
  /** "Good spot" reaction total, maintained server-side as a column. */
  good_spot_count: number;
  created_at: string;
  category: string | null;
  username: string | null;
  avatar_url: string | null;
  photos: PinPhoto[];
  /** Whether the viewer already reacted. Only GET /pins/:id answers it. */
  reacted_by_me?: boolean;
}

export interface NewPinPhoto {
  photo_url: string;
  thumbnail_url: string;
  position: number;
}

export interface CreatedPin {
  id: string;
  user_id: string | null;
  location: string;
  geohash: string;
  caption: string | null;
  category_id: number;
  is_hidden: boolean;
  views: number;
  /** A brand-new pin starts at 0 reactions. */
  good_spot_count: number;
  created_at: string;
}
