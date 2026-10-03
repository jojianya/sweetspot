import { describe, expect, it } from "vitest";
import { toPinListEntry } from "./pinEntry";
import type { CreatedPin, NewPinPhoto } from "@/lib/types";

const pin: CreatedPin = {
  id: "pin-1",
  user_id: "user-1",
  location: "POINT(78.4867 17.385)",
  geohash: "td1",
  caption: "A great spot",
  category_id: 2,
  is_hidden: false,
  views: 0,
  created_at: "2026-01-02T00:00:00Z",
};

function photo(overrides: Partial<NewPinPhoto> = {}): NewPinPhoto {
  return {
    photo_url: "https://media.example/full.webp",
    thumbnail_url: "https://media.example/thumb.webp",
    position: 0,
    ...overrides,
  };
}

describe("toPinListEntry", () => {
  it("prefers the thumbnail for the cover", () => {
    expect(toPinListEntry(pin, [photo()], "alice").cover_url).toBe(
      "https://media.example/thumb.webp"
    );
  });

  it("falls back to the full photo URL and then to empty", () => {
    const legacy = { ...photo(), thumbnail_url: undefined as unknown as string };
    expect(toPinListEntry(pin, [legacy], "alice").cover_url).toBe(
      "https://media.example/full.webp"
    );
    expect(toPinListEntry(pin, [], "alice").cover_url).toBe("");
  });

  it("copies the pin fields and the author name", () => {
    expect(toPinListEntry(pin, [photo()], "alice")).toEqual({
      id: "pin-1",
      user_id: "user-1",
      location: "POINT(78.4867 17.385)",
      geohash: "td1",
      caption: "A great spot",
      category_id: 2,
      is_hidden: false,
      views: 0,
      created_at: "2026-01-02T00:00:00Z",
      cover_url: "https://media.example/thumb.webp",
      username: "alice",
    });
  });
});
