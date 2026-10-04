import type { CreatedPin, NewPinPhoto, PinListEntry } from "@/lib/types";

/**
 * Assembles the list entry for a just-created pin so it can merge into the
 * viewport list without refetching. The cover prefers the thumbnail, then
 * the full photo URL, then empty. Extracted unchanged from MapApp.
 */
export function toPinListEntry(
  pin: CreatedPin,
  photos: NewPinPhoto[],
  username: string
): PinListEntry {
  const cover = photos[0]?.thumbnail_url ?? photos[0]?.photo_url ?? "";
  return {
    id: pin.id,
    user_id: pin.user_id,
    location: pin.location,
    geohash: pin.geohash,
    caption: pin.caption,
    category_id: pin.category_id,
    is_hidden: pin.is_hidden,
    views: pin.views,
    created_at: pin.created_at,
    cover_url: cover,
    username,
  };
}
