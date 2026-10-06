// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/hooks/useFavorites", () => ({
  useFavorites: () => ({
    entries: [
      {
        id: "pin-1",
        cover_url: "javascript:alert(1)",
        caption: "bad cover",
        username: "alice",
        created_at: "2026-01-02T00:00:00Z",
      },
    ],
    loading: false,
    error: null,
    retry: vi.fn(),
  }),
}));

vi.mock("@/hooks/useCollections", () => ({
  useCollections: () => ({
    collections: [
      {
        id: "col-1",
        name: "Rejected covers",
        cover_url: "data:text/html,<b>x</b>",
        pin_count: 2,
        created_at: "2026-01-02T00:00:00Z",
      },
    ],
    loading: false,
    error: null,
    retry: vi.fn(),
    create: vi.fn(),
  }),
}));

vi.mock("@/hooks/useCollectionDetail", () => ({
  useCollectionDetail: () => ({
    openCollection: null,
    setOpenCollection: vi.fn(),
    openingId: null,
    collectionError: null,
    setCollectionError: vi.fn(),
    togglingPrivate: false,
    openCollectionDetail: vi.fn(),
    handleRemovePin: vi.fn(),
    handleTogglePrivate: vi.fn(),
  }),
}));

vi.mock("@/store/auth", () => ({
  useAuth: () => ({ user: null }),
}));

import SavedPinsPanel from "./SavedPinsPanel";

describe("SavedPinsPanel rejected covers", () => {
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

  it("renders placeholder, not <img src=\"\">, for rejected saved cover", async () => {
    await act(async () => {
      root.render(<SavedPinsPanel onClose={() => {}} onOpenPin={() => {}} activeId={null} />);
    });
    const bad = Array.from(container.querySelectorAll("img")).filter((img) => {
      const src = img.getAttribute("src");
      return !src || !src.trim();
    });
    expect(bad).toHaveLength(0);
    expect(container.querySelector("img")).toBeNull();
  });

  it("renders placeholder in collections tab for rejected collection cover", async () => {
    await act(async () => {
      root.render(<SavedPinsPanel onClose={() => {}} onOpenPin={() => {}} activeId={null} />);
    });
    const tab = Array.from(container.querySelectorAll("button")).find((b) =>
      b.textContent?.toLowerCase().includes("collection")
    );
    expect(tab).not.toBeNull();
    await act(async () => {
      tab!.click();
    });
    const bad = Array.from(container.querySelectorAll("img")).filter((img) => {
      const src = img.getAttribute("src");
      return !src || !src.trim();
    });
    expect(bad).toHaveLength(0);
  });
});
