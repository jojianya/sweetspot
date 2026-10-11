// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PinDetail } from "@/lib/types";
import { useAuth } from "@/store/auth";

const makePin = (userId: string | null): PinDetail => ({
  id: "pin-1",
  user_id: userId,
  location: "POINT(-122.4194 37.7749)",
  geohash: "9q8yy",
  caption: "Test pin",
  category_id: 1,
  is_hidden: false,
  views: 42,
  good_spot_count: 0,
  created_at: "2024-01-15T10:00:00Z",
  category: "Food",
  username: userId ? "alice" : null,
  avatar_url: null,
  photos: [],
});

vi.mock("@/hooks/usePinView", () => ({
  usePinView: () => null,
}));
vi.mock("@/hooks/usePinAddress", () => ({
  usePinAddress: () => "San Francisco, CA",
}));
vi.mock("@/hooks/usePinShare", () => ({
  usePinShare: () => ({ copied: false, handleShare: vi.fn(), goDirections: vi.fn() }),
}));
vi.mock("@/hooks/useOptimisticSave", () => ({
  useOptimisticSave: () => ({ saved: false, saving: false, error: null, handleSave: vi.fn() }),
}));
vi.mock("@/hooks/useReaction", () => ({
  // Mocked for the same reason useOptimisticSave is: the real hook reaches for
  // the app router, which this plain createRoot harness does not mount.
  useReaction: () => ({ reacted: false, count: 0, busy: false, error: null, toggle: vi.fn() }),
}));
vi.mock("@/hooks/useCategories", () => ({
  useCategories: () => ({ categories: [{ id: 1, name: "Food", slug: "food" }] }),
}));

describe("PinDetailPanel with nullable user_id", () => {
  let container: HTMLDivElement;
  let root: Root;

  function userLinks(): HTMLAnchorElement[] {
    return Array.from(container.querySelectorAll('a[href^="/users/"]')) as HTMLAnchorElement[];
  }

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    useAuth.setState({ user: null });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  async function render(pin: PinDetail) {
    const PinDetailPanel = (await import("./PinDetailPanel")).default;
    await act(async () => {
      root.render(<PinDetailPanel pin={pin} onClose={vi.fn()} onDeleted={vi.fn()} />);
    });
  }

  it("renders without crash when pin has null user_id", async () => {
    await render(makePin(null));
    expect(container.textContent).toContain("Test pin");
    expect(container.textContent).not.toContain("alice");
    expect(container.textContent).toContain("deleted user");
  });

  it("does not render a link to /users/null when user_id is null", async () => {
    await render(makePin(null));
    expect(userLinks()).toHaveLength(0);
  });

  it("renders username link when user_id is present", async () => {
    await render(makePin("user-1"));
    const links = userLinks();
    expect(links).toHaveLength(1);
    expect(links[0].getAttribute("href")).toBe("/users/user-1");
    expect(links[0].textContent).toContain("alice");
  });

  it("shows [deleted user] label when username is null", async () => {
    await render(makePin(null));
    expect(container.textContent).toContain("deleted user");
  });
});