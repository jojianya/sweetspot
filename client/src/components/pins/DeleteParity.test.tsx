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
vi.mock("./PhotoLightbox", () => ({ default: () => null }));

const pin: PinDetail = {
  id: "pin-1",
  user_id: "owner-1",
  location: "POINT(0 0)",
  geohash: "s000",
  caption: "A great spot",
  category_id: 1,
  is_hidden: false,
  views: 4,
  good_spot_count: 0,
  created_at: "2026-01-02T00:00:00Z",
  category: "Food",
  username: "alice",
  avatar_url: null,
  photos: [],
};

function editButton(container: HTMLElement): HTMLButtonElement | null {
  // Side panel labels it "Edit", the permalink page "Edit pin".
  const buttons = Array.from(container.querySelectorAll("button"));
  return (
    (buttons.find((b) => b.textContent === "Edit" || b.textContent === "Edit pin") as HTMLButtonElement) ??
    null
  );
}

// The delete control lives inside the edit sheet, so edit-button visibility
// is delete-control visibility. Both surfaces must agree: owner and
// moderator see it, strangers and logged-out visitors do not.
describe.each([
  ["side panel", (c: HTMLElement, noop: () => void) => <PinDetailPanel pin={pin} onClose={noop} onDeleted={noop} />],
  ["permalink page", () => <PinPageClient initialPin={pin} />],
])("delete parity on %s", (_label, renderBody) => {
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
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  async function renderAs(role: "user" | "admin" | "owner" | null, userId: string | null) {
    useAuth.setState({
      user:
        role === null || userId === null
          ? null
          : { id: userId, username: "someone", avatar_url: null, role },
    });
    await act(async () => {
      root.render(renderBody(container, noop));
    });
  }

  it("owner sees Edit pin", async () => {
    await renderAs("user", "owner-1");
    expect(editButton(container)).not.toBeNull();
  });

  it("moderator sees Edit pin", async () => {
    await renderAs("admin", "moderator-9");
    expect(editButton(container)).not.toBeNull();
  });

  it("stranger does not see Edit pin", async () => {
    await renderAs("user", "stranger-2");
    expect(editButton(container)).toBeNull();
  });

  it("logged-out visitor does not see Edit pin", async () => {
    await renderAs(null, null);
    expect(editButton(container)).toBeNull();
  });
});
