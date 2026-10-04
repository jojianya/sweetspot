// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PinDetail } from "@/lib/types";
import { useAuth } from "@/store/auth";
import PinDetailPanel from "./PinDetailPanel";

const apiMocks = vi.hoisted(() => ({
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
  registerPinView: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

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

vi.mock("./PinEditSheet", () => ({ default: () => null }));
vi.mock("./AddToCollectionSheet", () => ({ default: () => null }));
vi.mock("./ReportSheet", () => ({ default: () => null }));
vi.mock("./PhotoLightbox", () => ({ default: () => null }));

function pin(id: string, views: number): PinDetail {
  return {
    id,
    user_id: "owner-9",
    location: "POINT(0 0)",
    geohash: "s000",
    caption: "A spot",
    category_id: 1,
    is_hidden: false,
    views,
    created_at: "2026-01-02T00:00:00Z",
    category: "Food",
    username: "alice",
    avatar_url: null,
    photos: [],
  };
}

function deferred() {
  let resolve!: (value: number) => void;
  const promise = new Promise<number>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe("PinDetailPanel view count", () => {
  let container: HTMLDivElement;
  let root: Root;
  const noop = () => {};
  let viewA: ReturnType<typeof deferred>;
  let viewB: ReturnType<typeof deferred>;

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
    viewA = deferred();
    viewB = deferred();
    apiMocks.registerPinView.mockReset().mockImplementation((id: string) => {
      if (id === "pin-a") return viewA.promise;
      if (id === "pin-b") return viewB.promise;
      return Promise.resolve(0);
    });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  it("keeps B's label when A's late response resolves after switching", async () => {
    await act(async () => {
      root.render(
        createElement(PinDetailPanel, { pin: pin("pin-a", 10), onClose: noop, onDeleted: noop })
      );
    });
    expect(container.textContent).toContain("10 views");

    // Navigate to pin B: a fresh mount, like the map's key={detail.id} remount.
    await act(async () => {
      root.unmount();
    });
    container.remove();
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    await act(async () => {
      root.render(
        createElement(PinDetailPanel, { pin: pin("pin-b", 20), onClose: noop, onDeleted: noop })
      );
    });
    expect(container.textContent).toContain("20 views");

    await act(async () => {
      viewA.resolve(99);
      await viewA.promise;
    });
    expect(container.textContent).toContain("20 views");
    expect(container.textContent).not.toContain("99 views");

    await act(async () => {
      viewB.resolve(21);
      await viewB.promise;
    });
    expect(container.textContent).toContain("21 views");
  });
});
