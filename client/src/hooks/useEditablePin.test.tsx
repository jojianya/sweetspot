// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PinDetail, PinPhoto } from "@/lib/types";
import { useAuth } from "@/store/auth";
import { useEditablePin } from "@/hooks/useEditablePin";

const apiMocks = vi.hoisted(() => ({
  registerPinView: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

function photo(id: string, base: string): PinPhoto {
  return {
    id,
    pin_id: "pin-1",
    photo_url: `/uploads/${base}.webp`,
    thumbnail_url: `/uploads/${base}-thumb.webp`,
    position: 0,
    created_at: "2026-01-02T00:00:00Z",
  };
}

function makePin(overrides: Partial<PinDetail> = {}): PinDetail {
  return {
    id: "pin-1",
    user_id: "owner-1",
    location: "POINT(0 0)",
    geohash: "s000",
    caption: "A great spot",
    category_id: 1,
    is_hidden: false,
    views: 42,
    good_spot_count: 0,
    created_at: "2026-01-02T00:00:00Z",
    category: "Food",
    username: "alice",
    avatar_url: null,
    photos: [photo("photo-1", "p1"), photo("photo-2", "p2")],
    ...overrides,
  };
}

function ViewDisplay({ pin }: { pin: PinDetail }) {
  const { pin: p } = useEditablePin(pin);
  return <span data-testid="views">{p.views}</span>;
}

describe("useEditablePin view count monotonicity", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.registerPinView.mockReset();
    useAuth.setState({
      user: { id: "viewer-1", username: "viewer", avatar_url: null, role: "user" },
    });
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

  it("increases view count when API returns higher value", async () => {
    apiMocks.registerPinView.mockResolvedValue(50); // higher than initial 42
    await act(async () => {
      root.render(createElement(ViewDisplay, { pin: makePin({ views: 42 }) }));
    });
    // Should show the higher value from API
    expect(container.textContent).toBe("50");
  });

  it("does not decrease view count when API returns lower value (monotonic)", async () => {
    apiMocks.registerPinView.mockResolvedValue(10); // lower than initial 42
    await act(async () => {
      root.render(createElement(ViewDisplay, { pin: makePin({ views: 42 }) }));
    });
    // Should keep the higher initial value (42), not decrease to 10
    expect(container.textContent).toBe("42");
  });

  it("keeps view count when API returns same value", async () => {
    apiMocks.registerPinView.mockResolvedValue(42); // same as initial
    await act(async () => {
      root.render(createElement(ViewDisplay, { pin: makePin({ views: 42 }) }));
    });
    expect(container.textContent).toBe("42");
  });
});

function IndexDisplay({ pin }: { pin: PinDetail }) {
  const { index, setIndex, handleUpdated } = useEditablePin(pin);
  return (
    <div>
      <span data-testid="index">{index}</span>
      <button
        data-testid="set-index"
        onClick={() => setIndex(2)}
      >
        set index
      </button>
      <button
        data-testid="update-pin"
        onClick={() => handleUpdated(makePin({ photos: [photo("p1", "p1"), photo("p2", "p2")] }))}
      >
        update
      </button>
    </div>
  );
}

describe("useEditablePin handleUpdated clamps index", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.registerPinView.mockReset().mockResolvedValue(42);
    useAuth.setState({
      user: { id: "viewer-1", username: "viewer", avatar_url: null, role: "user" },
    });
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

  it("clamps index when photos are reduced", async () => {
    // Initial pin has 3 photos, index starts at 0
    await act(async () => {
      root.render(createElement(IndexDisplay, { pin: makePin({ photos: [photo("p1", "p1"), photo("p2", "p2"), photo("p3", "p3")] }) }));
    });

    // Click set index button to go to index 2
    const setIndexBtn = container.querySelector('[data-testid="set-index"]') as HTMLButtonElement;
    await act(async () => {
      setIndexBtn.click();
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });
    expect(container.querySelector('[data-testid="index"]')?.textContent).toBe("2");

    // Click update button to reduce to 2 photos
    const updateBtn = container.querySelector('[data-testid="update-pin"]') as HTMLButtonElement;
    await act(async () => {
      updateBtn.click();
    });
    // Wait for index to be clamped
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10));
    });
    // Index should be clamped to 1 (last valid index for 2 photos)
    expect(container.querySelector('[data-testid="index"]')?.textContent).toBe("1");
  });
});