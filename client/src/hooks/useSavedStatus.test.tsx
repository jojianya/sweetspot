// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useSavedStatus } from "./useFavorites";

const apiMocks = vi.hoisted(() => ({
  fetchFavoriteIDs: vi.fn(),
  fetchFavorites: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function SavedProbe({ pinId, enabled }: { pinId: string; enabled: boolean }) {
  const { saved } = useSavedStatus(pinId, enabled);
  return <span>{saved ? "saved" : "unsaved"}</span>;
}

describe("useSavedStatus", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.fetchFavoriteIDs.mockReset().mockResolvedValue([]);
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

  function text(): string {
    return container.textContent ?? "";
  }

  it("reports saved for a pin in the id list", async () => {
    apiMocks.fetchFavoriteIDs.mockResolvedValue(["pin-1"]);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    expect(text()).toBe("saved");
  });

  it("does not fetch while disabled", async () => {
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: false }));
    });
    expect(apiMocks.fetchFavoriteIDs).not.toHaveBeenCalled();
    expect(text()).toBe("unsaved");
  });

  // The badge belongs to one pin: switching pins must not show the previous
  // pin's answer, and a late lookup for the old pin must not win.
  it("resets when the pin changes", async () => {
    apiMocks.fetchFavoriteIDs.mockResolvedValue(["pin-1"]);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    expect(text()).toBe("saved");

    // pin-2 is not in the list, so the pending lookup will answer "unsaved",
    // but the stale "saved" must already be gone on this render.
    apiMocks.fetchFavoriteIDs.mockResolvedValue([]);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-2", enabled: true }));
    });
    expect(text()).toBe("unsaved");
  });

  it("resets when disabled after being enabled", async () => {
    apiMocks.fetchFavoriteIDs.mockResolvedValue(["pin-1"]);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    expect(text()).toBe("saved");

    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: false }));
    });
    expect(text()).toBe("unsaved");
  });

  it("ignores a superseded lookup that resolves last", async () => {
    const first = deferred<string[]>();
    apiMocks.fetchFavoriteIDs.mockImplementationOnce(() => first.promise);
    apiMocks.fetchFavoriteIDs.mockResolvedValueOnce(["pin-2"]);

    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-2", enabled: true }));
    });
    await act(async () => {
      first.resolve(["pin-1"]);
      await first.promise;
    });
    expect(text()).toBe("saved"); // pin-2 is in its own list
  });
});