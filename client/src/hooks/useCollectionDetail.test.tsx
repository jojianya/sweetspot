// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CollectionDetail } from "@/lib/types";
import { useCollectionDetail } from "./useCollectionDetail";

const apiMocks = vi.hoisted(() => ({
  fetchCollection: vi.fn(),
  updateCollection: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

const collectionsMocks = vi.hoisted(() => ({
  removePin: vi.fn(),
  patchLocal: vi.fn(),
}));

vi.mock("@/hooks/useCollections", () => ({
  useCollections: () => ({
    collections: [],
    loading: false,
    error: null,
    retry: vi.fn(),
    create: vi.fn(),
    addPin: vi.fn(),
    ...collectionsMocks,
  }),
}));

function detail(overrides: Partial<CollectionDetail> = {}): CollectionDetail {
  return {
    id: "col-1",
    user_id: "user-1",
    name: "Secret",
    description: null,
    cover_url: "",
    is_private: true,
    pin_count: 2,
    created_at: "2026-01-01T00:00:00Z",
    pins: [
      {
        id: "pin-1",
        user_id: "user-1",
        location: "POINT(0 0)",
        geohash: "s000",
        caption: "One",
        category_id: 1,
        is_hidden: false,
        views: 0,
        good_spot_count: 0,
        created_at: "2026-01-02T00:00:00Z",
        cover_url: "",
        username: "alice",
      },
      {
        id: "pin-2",
        user_id: "user-1",
        location: "POINT(1 1)",
        geohash: "s001",
        caption: "Two",
        category_id: 1,
        is_hidden: false,
        views: 0,
        good_spot_count: 0,
        created_at: "2026-01-03T00:00:00Z",
        cover_url: "",
        username: "alice",
      },
    ],
    ...overrides,
  };
}

function DetailProbe() {
  const {
    openCollection,
    openingId,
    collectionError,
    togglingPrivate,
    openCollectionDetail,
    handleRemovePin,
    handleTogglePrivate,
  } = useCollectionDetail();
  return (
    <>
      <span data-testid="state">
        {openCollection ? `${openCollection.id}:${openCollection.pins.length}:${openCollection.pin_count}:${openCollection.is_private ? "private" : "public"}` : "closed"}
        :{openingId ?? "idle"}:{collectionError ?? "noerror"}:{togglingPrivate ? "busy" : "free"}
      </span>
      <button type="button" data-testid="open" onClick={() => void openCollectionDetail("col-1")}>
        open
      </button>
      <button
        type="button"
        data-testid="remove"
        onClick={() => void handleRemovePin("col-1", "pin-1")}
      >
        remove
      </button>
      <button type="button" data-testid="toggle" onClick={() => void handleTogglePrivate(false)}>
        toggle
      </button>
    </>
  );
}

describe("useCollectionDetail", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.fetchCollection.mockReset().mockResolvedValue(detail());
    apiMocks.updateCollection.mockReset().mockResolvedValue(undefined);
    collectionsMocks.removePin.mockReset().mockResolvedValue(undefined);
    collectionsMocks.patchLocal.mockReset();
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

  function state(): string {
    return container.querySelector('[data-testid="state"]')?.textContent ?? "";
  }

  function click(testId: string): void {
    (container.querySelector(`[data-testid="${testId}"]`) as HTMLButtonElement).click();
  }

  async function renderProbe(): Promise<void> {
    await act(async () => {
      root.render(createElement(DetailProbe));
    });
  }

  it("opens a collection and clears the opening flag", async () => {
    await renderProbe();
    await act(async () => {
      click("open");
    });
    expect(apiMocks.fetchCollection).toHaveBeenCalledWith("col-1");
    expect(state()).toContain("col-1:2:2:private:idle:noerror:free");
  });

  it("surfaces open failures", async () => {
    apiMocks.fetchCollection.mockRejectedValue(new Error("gone"));
    await renderProbe();
    await act(async () => {
      click("open");
    });
    expect(state()).toContain("closed:idle:gone:free");
  });

  it("removes a pin optimistically with pin_count - 1", async () => {
    await renderProbe();
    await act(async () => {
      click("open");
    });
    await act(async () => {
      click("remove");
    });
    expect(collectionsMocks.removePin).toHaveBeenCalledWith("col-1", "pin-1");
    expect(state()).toContain("col-1:1:1:private");
  });

  it("refetches to roll back when removal fails and keeps the error visible", async () => {
    collectionsMocks.removePin.mockRejectedValueOnce(new Error("offline"));
    await renderProbe();
    await act(async () => {
      click("open");
    });
    const refetched = detail();
    apiMocks.fetchCollection.mockResolvedValue(refetched);
    await act(async () => {
      click("remove");
    });
    expect(apiMocks.fetchCollection).toHaveBeenCalledTimes(2);
    // The rollback refetch restores the pin list, but the removal failure
    // stays visible until the next successful action clears it.
    expect(state()).toContain("col-1:2:2:private");
    expect(state()).toContain("offline");
  });

  it("toggles privacy locally and in the list", async () => {
    await renderProbe();
    await act(async () => {
      click("open");
    });
    await act(async () => {
      click("toggle");
    });
    expect(apiMocks.updateCollection).toHaveBeenCalledWith("col-1", {
      name: "Secret",
      description: null,
      isPrivate: false,
    });
    expect(collectionsMocks.patchLocal).toHaveBeenCalledWith("col-1", {
      is_private: false,
    });
    expect(state()).toContain("col-1:2:2:public");
  });

  it("surfaces toggle failures without patching", async () => {
    apiMocks.updateCollection.mockRejectedValue(new Error("forbidden"));
    await renderProbe();
    await act(async () => {
      click("open");
    });
    await act(async () => {
      click("toggle");
    });
    expect(collectionsMocks.patchLocal).not.toHaveBeenCalled();
    expect(state()).toContain("col-1:2:2:private");
    expect(state()).toContain("forbidden");
  });
});
