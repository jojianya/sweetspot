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

const emptyPhotoPin: PinDetail = {
  id: "pin-1",
  user_id: "user-1",
  location: "POINT(0 0)",
  geohash: "s000",
  caption: "hi",
  category_id: 1,
  is_hidden: false,
  views: 0,
  created_at: "2026-01-02T00:00:00Z",
  category: "Food",
  username: "alice",
  avatar_url: null,
  photos: [
    {
      id: "photo-1",
      pin_id: "pin-1",
      photo_url: "",
      thumbnail_url: "",
      position: 0,
      created_at: "2026-01-02T00:00:00Z",
    },
  ],
};

describe("PinEditSheet empty src guard (H2)", () => {
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

  it("never renders <img src=\"\"> for empty photo URLs", async () => {
    await act(async () => {
      root.render(
        <PinEditSheet
          pin={emptyPhotoPin}
          categories={[{ id: 1, name: "Food", slug: "food" }]}
          onClose={() => {}}
          onUpdated={() => {}}
          onDeleted={() => {}}
        />
      );
    });
    // React omits empty src entirely (no attribute), which still renders a
    // broken image. Any img without a usable src is a failure.
    const bad = Array.from(container.querySelectorAll("img")).filter((img) => {
      const src = img.getAttribute("src");
      return !src || !src.trim();
    });
    expect(bad).toHaveLength(0);
  });
});
