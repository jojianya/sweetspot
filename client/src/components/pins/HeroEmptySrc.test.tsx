// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PinDetail } from "@/lib/types";
import { useAuth } from "@/store/auth";
import PinDetailPanel from "./PinDetailPanel";
import PinPageClient from "./PinPageClient";

vi.mock("@/lib/api", () => ({
  fetchPin: vi.fn(),
  fetchCategories: vi.fn().mockResolvedValue([]),
  fetchFavoriteIDs: vi.fn().mockResolvedValue([]),
  fetchFavorites: vi.fn().mockResolvedValue([]),
  saveFavorite: vi.fn(),
  removeFavorite: vi.fn(),
  fetchComments: vi.fn().mockResolvedValue([]),
  createComment: vi.fn(),
  deleteComment: vi.fn(),
  updatePin: vi.fn(),
  deletePin: vi.fn(),
  registerPinView: vi.fn().mockResolvedValue(1),
}));

vi.mock("@/lib/api/geocoding", () => ({
  reverseGeocode: vi.fn().mockResolvedValue(null),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("next/link", async () => {
  const { createElement } = await import("react");
  return {
    default: ({ href, children }: { href: string; children?: React.ReactNode }) =>
      createElement("a", { href }, children),
  };
});

vi.mock("@/components/layout/Navbar", () => ({ default: () => null }));
vi.mock("./PinEditSheet", () => ({ default: () => null }));
vi.mock("./AddToCollectionSheet", () => ({ default: () => null }));
vi.mock("./ReportSheet", () => ({ default: () => null }));
vi.mock("./PhotoLightbox", () => ({
  default: ({ src }: { src: string }) => <img src={src} alt="lightbox" />,
}));

function emptyPhotoPin(): PinDetail {
  return {
    id: "pin-1",
    user_id: "owner-1",
    location: "POINT(0 0)",
    geohash: "s000",
    caption: "A spot",
    category_id: 1,
    is_hidden: false,
    views: 4,
    good_spot_count: 0,
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
}

describe.each([
  ["side panel", (pin: PinDetail, noop: () => void) => <PinDetailPanel pin={pin} onClose={noop} onDeleted={noop} />],
  ["permalink page", (pin: PinDetail) => <PinPageClient initialPin={pin} />],
])("hero empty src on %s", (_label, renderBody) => {
  let container: HTMLDivElement;
  let root: Root;
  const noop = () => {};

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    useAuth.setState({
      user: { id: "viewer-1", username: "viewer", avatar_url: null, role: "user" },
    });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  it("renders placeholder, no <img> without usable src", async () => {
    await act(async () => {
      root.render(renderBody(emptyPhotoPin(), noop));
    });
    const bad = Array.from(container.querySelectorAll("img")).filter((img) => {
      const src = img.getAttribute("src");
      return !src || !src.trim();
    });
    expect(bad).toHaveLength(0);
    // Hero zoom button must be absent when there is no usable photo URL,
    // so the lightbox cannot open with an empty src.
    const zoom = Array.from(container.querySelectorAll("button")).find(
      (b) => b.getAttribute("aria-label") === "View photo full screen"
    );
    expect(zoom).toBeUndefined();
  });
});
