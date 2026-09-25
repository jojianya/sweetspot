import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PinDetail } from "@/lib/types";
import PinPage, { generateMetadata } from "./page";

const mocks = vi.hoisted(() => ({
  getPinServer: vi.fn(),
  notFound: vi.fn(),
}));

vi.mock("@/lib/api/server", () => ({ getPinServer: mocks.getPinServer }));
vi.mock("@/components/pins/PinPageClient", () => ({ default: () => null }));
vi.mock("@/lib/site", () => ({
  getSiteUrl: () => new URL("https://goodspot.example"),
}));
vi.mock("next/navigation", () => ({ notFound: mocks.notFound }));

const PIN_ID = "11111111-1111-4111-8111-111111111111";
const pin: PinDetail = {
  id: PIN_ID,
  user_id: "22222222-2222-4222-8222-222222222222",
  location: "POINT(0 0)",
  geohash: "s000",
  caption: "A great spot",
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
      pin_id: PIN_ID,
      photo_url: "https://media.example/pin.webp",
      thumbnail_url: "https://media.example/pin-thumb.webp",
      position: 0,
      created_at: "2026-01-02T00:00:00Z",
    },
  ],
};

describe("pin page metadata", () => {
  beforeEach(() => {
    mocks.getPinServer.mockReset().mockResolvedValue(pin);
    mocks.notFound.mockReset().mockImplementation(() => {
      throw new Error("NEXT_NOT_FOUND");
    });
  });

  it("returns complete share metadata for a found pin", async () => {
    const metadata = await generateMetadata({ params: Promise.resolve({ id: PIN_ID }) });

    expect(mocks.getPinServer).toHaveBeenCalledWith(PIN_ID);
    expect(metadata.title).toBe("A great spot · Goodspot");
    expect(metadata.alternates?.canonical).toBe(`https://goodspot.example/pin/${PIN_ID}`);
    expect(metadata.openGraph).toMatchObject({
      url: `https://goodspot.example/pin/${PIN_ID}`,
      siteName: "Goodspot",
    });
    expect(metadata.twitter).toMatchObject({ card: "summary_large_image" });
  });

  it("marks missing-pin metadata as noindex", async () => {
    mocks.getPinServer.mockResolvedValue(null);

    const metadata = await generateMetadata({ params: Promise.resolve({ id: PIN_ID }) });

    expect(metadata).toEqual({
      title: "Pin not found · Goodspot",
      robots: { index: false, follow: false },
    });
  });

  it("keeps the page not-found path for a missing pin", async () => {
    mocks.getPinServer.mockResolvedValue(null);

    await expect(PinPage({ params: Promise.resolve({ id: PIN_ID }) })).rejects.toThrow(
      "NEXT_NOT_FOUND"
    );
    expect(mocks.notFound).toHaveBeenCalledOnce();
  });
});
