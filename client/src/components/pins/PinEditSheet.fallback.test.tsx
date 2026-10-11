// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PinDetail } from "@/lib/types";
import PinEditSheet from "./PinEditSheet";

vi.mock("@/lib/api", () => ({
  updatePin: vi.fn(),
  deletePin: vi.fn(),
}));

function pinWithThumbFallback(): PinDetail {
  return {
    id: "pin-1",
    user_id: "user-1",
    location: "POINT(0 0)",
    geohash: "s000",
    caption: "hi",
    category_id: 1,
    is_hidden: false,
    views: 0,
    good_spot_count: 0,
    created_at: "2026-01-02T00:00:00Z",
    category: "Food",
    username: "alice",
    avatar_url: null,
    photos: [
      {
        id: "photo-1",
        pin_id: "pin-1",
        photo_url: "http://192.168.68.112:8081/uploads/full.webp",
        thumbnail_url: "",
        position: 0,
        created_at: "2026-01-02T00:00:00Z",
      },
    ],
  };
}

describe("PinEditSheet thumbnail fallback (M3)", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  it("falls back to photo_url when thumbnail_url is empty", async () => {
    await act(async () => {
      root.render(
        <PinEditSheet
          pin={pinWithThumbFallback()}
          categories={[{ id: 1, name: "Food", slug: "food" }]}
          onClose={() => {}}
          onUpdated={() => {}}
          onDeleted={() => {}}
        />
      );
    });
    const imgs = Array.from(container.querySelectorAll("img"));
    expect(imgs.length).toBeGreaterThan(0);
    expect(imgs[0].getAttribute("src")).toBe("/uploads/full.webp");
  });
});
