// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import MapApp from "./MapApp";

const holders = vi.hoisted(() => ({
  stream: {} as {
    onPinRemoved?: (id: string) => void;
  },
  pinsState: {
    pins: [
      { id: "pin-open" },
      { id: "pin-other" },
    ] as { id: string }[],
  },
  removePin: vi.fn((id: string) => {
    holders.pinsState.pins = holders.pinsState.pins.filter((p) => p.id !== id);
  }),
}));

vi.mock("@/hooks/usePins", () => ({
  usePins: () => ({
    pins: holders.pinsState.pins,
    loading: false,
    error: null,
    addPin: vi.fn(),
    removePin: holders.removePin,
    refetch: vi.fn(),
  }),
}));

vi.mock("@/hooks/usePinDetail", () => ({
  usePinDetail: (id: string | null) => ({ detail: id ? { id } : null }),
}));

vi.mock("@/hooks/useCategories", () => ({
  useCategories: () => ({ categories: [], error: null, retry: vi.fn() }),
}));

vi.mock("@/hooks/useTrending", () => ({
  useTrending: () => ({ pins: [], loading: false, error: null }),
}));

vi.mock("@/hooks/usePinStream", () => ({
  usePinStream: (
    _bbox: string | null,
    _category: number | null,
    _onPin: unknown,
    onPinRemoved?: (id: string) => void
  ) => {
    holders.stream.onPinRemoved = onPinRemoved;
  },
}));

vi.mock("@/store/auth", () => ({
  useAuth: Object.assign(() => ({ user: null }), {
    getState: () => ({ user: null }),
  }),
}));

vi.mock("@/store/theme", () => ({
  useTheme: () => ({ theme: "light" }),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  usePathname: () => "/",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("./MapView", () => ({
  default: (props: {
    onSelectPin: (id: string) => void;
    onBoundsChange: (bbox: string, c: { lat: number; lng: number }) => void;
  }) =>
    createElement(
      "div",
      null,
      createElement("button", {
        type: "button",
        "data-testid": "select-open",
        onClick: () => props.onSelectPin("pin-open"),
      }),
      createElement("button", {
        type: "button",
        "data-testid": "set-bounds",
        onClick: () => props.onBoundsChange("1,2,3,4", { lat: 0, lng: 0 }),
      })
    ),
}));

vi.mock("./MapNavBar", () => ({ default: () => null }));
vi.mock("./LocateButton", () => ({ default: () => null }));
vi.mock("@/components/pins/CreatePinButton", () => ({ default: () => null }));
vi.mock("@/components/pins/SavedPinsPanel", () => ({ default: () => null }));
vi.mock("@/components/pins/TrendingList", () => ({
  default: () => null,
  TrendingIcon: () => null,
}));
vi.mock("@/components/pins/PinDetailPanel", () => ({
  default: (props: { pin: { id: string } }) =>
    createElement("div", { "data-testid": "detail", "data-id": props.pin.id }),
}));

function click(testid: string) {
  const el = document.querySelector(`[data-testid="${testid}"]`);
  if (!el) throw new Error(`missing button ${testid}`);
  (el as HTMLButtonElement).click();
}

function detailOpen() {
  return document.querySelector('[data-testid="detail"]');
}

describe("MapApp pin_removed", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    holders.pinsState.pins = [{ id: "pin-open" }, { id: "pin-other" }];
    holders.removePin.mockClear();
    holders.stream.onPinRemoved = undefined;
    vi.useFakeTimers();
    const actEnv = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean };
    actEnv.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    vi.useRealTimers();
  });

  async function openDetail() {
    await act(async () => {
      root.render(createElement(MapApp, null));
    });
    await act(async () => {
      click("set-bounds");
      vi.advanceTimersByTime(300);
    });
    await act(async () => {
      click("select-open");
    });
    expect(detailOpen()).not.toBeNull();
    if (!holders.stream.onPinRemoved) throw new Error("usePinStream onPinRemoved not captured");
  }

  it("removal of the open pin closes the detail panel and removes it from the list", async () => {
    await openDetail();
    await act(async () => {
      holders.stream.onPinRemoved!("pin-open");
    });
    expect(holders.removePin).toHaveBeenCalledWith("pin-open");
    expect(detailOpen()).toBeNull();
    expect(holders.pinsState.pins.some((p) => p.id === "pin-open")).toBe(false);
  });

  it("removal of a different pin leaves the panel open", async () => {
    await openDetail();
    await act(async () => {
      holders.stream.onPinRemoved!("pin-other");
    });
    expect(holders.removePin).toHaveBeenCalledWith("pin-other");
    expect(detailOpen()).not.toBeNull();
    expect(detailOpen()?.getAttribute("data-id")).toBe("pin-open");
  });
});
