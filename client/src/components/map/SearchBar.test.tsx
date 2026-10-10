// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import SearchBar from "./SearchBar";

vi.mock("@/lib/api/geocoding", () => ({
  searchPlaces: vi.fn(),
}));
vi.mock("@/lib/api/pins", () => ({
  searchPins: vi.fn(),
}));

import { searchPlaces } from "@/lib/api/geocoding";
import { searchPins } from "@/lib/api/pins";

const places = [
  { id: "p1", text: "Place One", place_name: "Place One, Earth", center: { lat: 1, lng: 2 } },
  { id: "p2", text: "Place Two", place_name: "Place Two, Earth", center: { lat: 3, lng: 4 } },
];
const pins = [
  {
    id: "pin1", user_id: "u1", location: "POINT(2 1)", geohash: "x",
    caption: "Pin Alpha", category_id: 1, is_hidden: false, views: 0, good_spot_count: 0,
    created_at: "2026-01-01", cover_url: "", username: "alice",
  },
  {
    id: "pin2", user_id: "u2", location: "POINT(4 3)", geohash: "y",
    caption: "Pin Beta", category_id: 1, is_hidden: false, views: 0, good_spot_count: 0,
    created_at: "2026-01-02", cover_url: "", username: "bob",
  },
];

describe("SearchBar", () => {
  let container: HTMLDivElement;
  let root: Root;
  const center = { lat: 0, lng: 0 };

  beforeEach(() => {
    vi.useFakeTimers();
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    vi.mocked(searchPlaces).mockResolvedValue(places);
    vi.mocked(searchPins).mockResolvedValue(pins);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  async function typeQuery(value: string) {
    const input = container.querySelector("input")!;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
    setter.call(input, value);
    await act(async () => {
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => {
      vi.advanceTimersByTime(350);
      await Promise.resolve();
      await Promise.resolve();
    });
  }

  function options(): HTMLLIElement[] {
    return Array.from(container.querySelectorAll('[role="option"]'));
  }

  function pressKey(key: string) {
    const input = container.querySelector("input")!;
    return act(async () => {
      input.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
    });
  }

  it("names the search input and keeps the combobox pattern", async () => {
    await act(async () => {
      root.render(createElement(SearchBar, { center, onSelectPlace: vi.fn(), onSelectPin: vi.fn() }));
    });
    const input = container.querySelector("input")!;
    expect(input.getAttribute("aria-label")).toBeTruthy();
    expect(input.getAttribute("role")).toBe("combobox");
    expect(input.getAttribute("aria-expanded")).toBe("false");
    expect(input.getAttribute("aria-controls")).toBe("search-results-listbox");
    expect(input.getAttribute("aria-autocomplete")).toBe("list");
  });

  it("exposes the highlighted option through aria-activedescendant", async () => {
    await act(async () => {
      root.render(createElement(SearchBar, { center, onSelectPlace: vi.fn(), onSelectPin: vi.fn() }));
    });
    await typeQuery("place");

    const input = container.querySelector("input")!;
    expect(input.getAttribute("aria-activedescendant")).toBeNull();

    await pressKey("ArrowDown");
    const opts = options();
    expect(opts.length).toBeGreaterThan(0);
    const ids = opts.map((o) => o.id);
    expect(ids.every((id) => id !== "")).toBe(true);
    expect(new Set(ids).size).toBe(opts.length);
    const active = opts.find((o) => o.getAttribute("aria-selected") === "true")!;
    expect(input.getAttribute("aria-activedescendant")).toBe(active.id);
  });

  it("hovering a pin then pressing Enter opens that pin (merged index, not section-local)", async () => {
    const onSelectPin = vi.fn();
    await act(async () => {
      root.render(createElement(SearchBar, { center, onSelectPlace: vi.fn(), onSelectPin }));
    });
    await typeQuery("pin");

    const opts = options();
    expect(opts).toHaveLength(4); // 2 places + 2 pins
    const secondPin = opts[3]; // merged index 3: places.length(2) + 1

    await act(async () => {
      secondPin.dispatchEvent(new MouseEvent("mouseover", { bubbles: true }));
    });
    expect(secondPin.getAttribute("aria-selected")).toBe("true");

    await pressKey("Enter");
    expect(onSelectPin).toHaveBeenCalledTimes(1);
    expect(onSelectPin.mock.calls[0][0].id).toBe("pin2");
  });

  it("arrow keys highlight the same row that Enter selects", async () => {
    const onSelectPin = vi.fn();
    const onSelectPlace = vi.fn();
    await act(async () => {
      root.render(createElement(SearchBar, { center, onSelectPlace, onSelectPin }));
    });
    await typeQuery("place");

    await pressKey("ArrowDown"); // index 0: first place
    await pressKey("ArrowDown"); // index 1: second place
    await pressKey("ArrowDown"); // index 2: first pin
    const opts = options();
    expect(opts[2].getAttribute("aria-selected")).toBe("true");
    await pressKey("Enter");
    expect(onSelectPin).toHaveBeenCalledTimes(1);
    expect(onSelectPin.mock.calls[0][0].id).toBe("pin1");
  });
});
