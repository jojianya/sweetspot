import { describe, expect, it } from "vitest";
import type { PinDetail } from "@/lib/types";
import { buildPinMetadata, resolvePublicMediaUrl } from "./pin";

function pin(overrides: Partial<PinDetail> = {}): PinDetail {
  return {
    id: "11111111-1111-4111-8111-111111111111",
    user_id: "22222222-2222-4222-8222-222222222222",
    location: "POINT(0 0)",
    geohash: "s000",
    caption: "Sunset lookout",
    category_id: 1,
    is_hidden: false,
    views: 4,
    created_at: "2026-01-02T00:00:00Z",
    category: "Food",
    username: "alice",
    avatar_url: null,
    photos: [
      {
        id: "photo-1",
        pin_id: "11111111-1111-4111-8111-111111111111",
        photo_url: "https://media.example/pin.webp",
        thumbnail_url: "https://media.example/pin-thumb.webp",
        position: 0,
        created_at: "2026-01-02T00:00:00Z",
      },
    ],
    ...overrides,
  };
}

describe("buildPinMetadata", () => {
  it("builds canonical, Open Graph, and Twitter metadata from the first full photo", () => {
    const metadata = buildPinMetadata(
      pin(),
      new URL("https://goodspot.example"),
      "https://api.goodspot.example"
    );

    expect(metadata.title).toBe("Sunset lookout · Goodspot");
    expect(metadata.description).toContain("by @alice in Food");
    expect(metadata.alternates?.canonical).toBe(
      "https://goodspot.example/pin/11111111-1111-4111-8111-111111111111"
    );
    expect(metadata.openGraph).toMatchObject({
      type: "website",
      url: "https://goodspot.example/pin/11111111-1111-4111-8111-111111111111",
      siteName: "Goodspot",
      title: "Sunset lookout · Goodspot",
      images: [
        {
          url: "https://media.example/pin.webp",
          alt: "Sunset lookout — Food by @alice",
        },
      ],
    });
    expect(metadata.twitter).toMatchObject({
      card: "summary_large_image",
      title: "Sunset lookout · Goodspot",
      images: [
        {
          url: "https://media.example/pin.webp",
          alt: "Sunset lookout — Food by @alice",
        },
      ],
    });
    expect(metadata.twitter?.description).toBe(metadata.description);
    expect(metadata.openGraph?.images).not.toMatchObject([{ width: 1200, height: 630 }]);
  });

  it("uses category and summary-card fallbacks when caption and photo are absent", () => {
    const metadata = buildPinMetadata(
      pin({ caption: null, category: "Live Music", photos: [] }),
      new URL("https://goodspot.example"),
      "https://api.goodspot.example"
    );

    expect(metadata.title).toBe("Live Music on Goodspot");
    expect(metadata.description).toContain("Live Music shared by @alice");
    expect(metadata.openGraph).not.toHaveProperty("images");
    expect(metadata.twitter).toMatchObject({ card: "summary" });
    expect(metadata.twitter).not.toHaveProperty("images");
  });

  it("truncates long user-generated text", () => {
    const caption = "A".repeat(500);
    const metadata = buildPinMetadata(
      pin({ caption }),
      new URL("https://goodspot.example"),
      "https://api.goodspot.example"
    );

    expect(String(metadata.title)).toContain("… · Goodspot");
    expect(String(metadata.title).length).toBeLessThan(100);
    expect(String(metadata.description).length).toBeLessThanOrEqual(200);
  });
});

describe("resolvePublicMediaUrl", () => {
  it("resolves relative storage paths against the public API", () => {
    expect(resolvePublicMediaUrl("/uploads/pin.webp", "https://api.goodspot.example")).toBe(
      "https://api.goodspot.example/uploads/pin.webp"
    );
  });

  it("rewrites legacy localhost media URLs to the public API", () => {
    expect(resolvePublicMediaUrl("http://localhost:8081/uploads/pin.webp", "https://api.goodspot.example")).toBe(
      "https://api.goodspot.example/uploads/pin.webp"
    );
  });

  it("keeps local media URLs for local development", () => {
    expect(resolvePublicMediaUrl("http://localhost:8081/uploads/pin.webp", "http://localhost:8081")).toBe(
      "http://localhost:8081/uploads/pin.webp"
    );
  });

  it("omits unusable or unconfigured media URLs", () => {
    expect(resolvePublicMediaUrl("javascript:alert(1)", "https://api.goodspot.example")).toBeNull();
    expect(resolvePublicMediaUrl("/uploads/pin.webp", undefined)).toBeNull();
    expect(resolvePublicMediaUrl("http://localhost:8081/uploads/pin.webp", undefined)).toBeNull();
  });
});
