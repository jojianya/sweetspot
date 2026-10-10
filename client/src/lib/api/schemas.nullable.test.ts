import { describe, it, expect } from "vitest";
import {
  pinListEntrySchema,
  trendingPinSchema,
  pinDetailSchema,
  createdPinSchema,
  collectionDetailSchema,
  commentSchema,
  reportEntrySchema,
} from "./schemas";

describe("nullable user_id / reporter_id in schemas", () => {
  const basePin = {
    id: "pin-1",
    location: "POINT(-122.4194 37.7749)",
    geohash: "9q8yy",
    caption: "Test pin",
    category_id: 1,
    is_hidden: false,
    views: 42,
    good_spot_count: 0,
    created_at: "2024-01-15T10:00:00Z",
  };

  it("parses pin list entry with null user_id", () => {
    const input = { ...basePin, user_id: null, cover_url: "https://example.com/cover.jpg", username: "alice" };
    const result = pinListEntrySchema.parse(input);
    expect(result.user_id).toBeNull();
    expect(result.username).toBe("alice");
  });

  it("parses trending pin with null user_id", () => {
    const input = { ...basePin, user_id: null, cover_url: "https://example.com/cover.jpg", username: null, comment_count: 5, score: 100 };
    const result = trendingPinSchema.parse(input);
    expect(result.user_id).toBeNull();
  });

  it("parses pin detail with null user_id", () => {
    const input = { ...basePin, user_id: null, category: "Food", username: "bob", avatar_url: null, photos: [] };
    const result = pinDetailSchema.parse(input);
    expect(result.user_id).toBeNull();
  });

  it("parses created pin with null user_id", () => {
    const input = { ...basePin, user_id: null };
    const result = createdPinSchema.parse(input);
    expect(result.user_id).toBeNull();
  });

  it("parses collection detail pins array with null user_id", () => {
    const input = {
      id: "coll-1",
      user_id: "user-1",
      name: "My Collection",
      description: null,
      is_private: false,
      created_at: "2024-01-15T10:00:00Z",
      pin_count: 1,
      cover_url: "https://example.com/cover.jpg",
      pins: [{ ...basePin, user_id: null, cover_url: "https://example.com/cover.jpg", username: null }],
    };
    const result = collectionDetailSchema.parse(input);
    expect(result.pins[0].user_id).toBeNull();
  });

  it("parses comments list with one null-author comment among normal ones", () => {
    const comments = [
      { id: "c1", pin_id: "pin-1", user_id: "user-1", body: "Great pin!", is_hidden: false, created_at: "2024-01-15T10:00:00Z", username: "alice", avatar_url: null },
      { id: "c2", pin_id: "pin-1", user_id: null, body: "Orphan comment", is_hidden: false, created_at: "2024-01-15T11:00:00Z", username: null, avatar_url: null },
      { id: "c3", pin_id: "pin-1", user_id: "user-2", body: "Another one", is_hidden: false, created_at: "2024-01-15T12:00:00Z", username: "bob", avatar_url: null },
    ];
    const result = commentSchema.array().parse(comments);
    expect(result).toHaveLength(3);
    expect(result[0].user_id).toBe("user-1");
    expect(result[1].user_id).toBeNull();
    expect(result[2].user_id).toBe("user-2");
    expect(result[1].username).toBeNull();
  });

  it("parses report list with null reporter_id", () => {
    const reports = [
      {
        id: "r1",
        pin_id: "pin-1",
        reporter_id: "user-1",
        reason: "Spam",
        status: "pending" as const,
        resolved_by: null,
        resolved_at: null,
        created_at: "2024-01-15T10:00:00Z",
        reporter_username: "alice",
        pin_caption: "Test pin",
      },
      {
        id: "r2",
        pin_id: "pin-2",
        reporter_id: null,
        reason: "Inappropriate",
        status: "pending" as const,
        resolved_by: null,
        resolved_at: null,
        created_at: "2024-01-15T11:00:00Z",
        reporter_username: null,
        pin_caption: "Another pin",
      },
    ];
    const result = reportEntrySchema.array().parse(reports);
    expect(result).toHaveLength(2);
    expect(result[0].reporter_id).toBe("user-1");
    expect(result[1].reporter_id).toBeNull();
    expect(result[1].reporter_username).toBeNull();
  });
});