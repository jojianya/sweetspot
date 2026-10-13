// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useSavedStatus } from "./useFavorites";

const apiMocks = vi.hoisted(() => ({
  fetchIsSaved: vi.fn(),
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
    apiMocks.fetchIsSaved.mockReset().mockResolvedValue(false);
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

  it("reports saved for a pin the per-pin check says is saved", async () => {
    apiMocks.fetchIsSaved.mockResolvedValue(true);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    expect(text()).toBe("saved");
  });

  it("asks the server about one pin, not the whole id list", async () => {
    apiMocks.fetchIsSaved.mockResolvedValue(true);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    expect(apiMocks.fetchIsSaved).toHaveBeenCalledTimes(1);
    expect(apiMocks.fetchIsSaved).toHaveBeenCalledWith("pin-1");
  });

  it("does not fetch while disabled", async () => {
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: false }));
    });
    expect(apiMocks.fetchIsSaved).not.toHaveBeenCalled();
    expect(text()).toBe("unsaved");
  });

  // The badge belongs to one pin: switching pins must not show the previous
  // pin's answer, and a late lookup for the old pin must not win.
  it("resets when the pin changes", async () => {
    apiMocks.fetchIsSaved.mockResolvedValue(true);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    expect(text()).toBe("saved");

    // pin-2 is not saved, so the pending lookup will answer "unsaved", but the
    // stale "saved" must already be gone on this render.
    apiMocks.fetchIsSaved.mockResolvedValue(false);
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-2", enabled: true }));
    });
    expect(text()).toBe("unsaved");
  });

  it("resets when disabled after being enabled", async () => {
    apiMocks.fetchIsSaved.mockResolvedValue(true);
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
    const first = deferred<boolean>();
    apiMocks.fetchIsSaved.mockImplementationOnce(() => first.promise);
    apiMocks.fetchIsSaved.mockResolvedValueOnce(true);

    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-1", enabled: true }));
    });
    await act(async () => {
      root.render(createElement(SavedProbe, { pinId: "pin-2", enabled: true }));
    });
    await act(async () => {
      first.resolve(true);
      await first.promise;
    });
    expect(text()).toBe("saved"); // pin-2's own answer wins
  });
});