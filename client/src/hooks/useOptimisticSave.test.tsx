// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";
import { useOptimisticSave } from "./useOptimisticSave";

const apiMocks = vi.hoisted(() => ({
  saveFavorite: vi.fn(),
  removeFavorite: vi.fn(),
  fetchFavoriteIDs: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

const routerMocks = vi.hoisted(() => ({
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: routerMocks.push }),
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function SaveProbe({ pinId }: { pinId: string }) {
  const { saved, saving, handleSave } = useOptimisticSave(pinId);
  return (
    <button type="button" disabled={saving} onClick={handleSave}>
      {saved ? "saved" : "unsaved"}:{saving ? "busy" : "idle"}
    </button>
  );
}

function loggedIn() {
  useAuth.setState({
    user: { id: "user-1", username: "someone", avatar_url: null, role: "user" },
  });
}

describe("useOptimisticSave", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.saveFavorite.mockReset().mockResolvedValue(undefined);
    apiMocks.removeFavorite.mockReset().mockResolvedValue(undefined);
    apiMocks.fetchFavoriteIDs.mockReset().mockResolvedValue([]);
    routerMocks.push.mockReset();
    useAuth.setState({ user: null });
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

  it("sends logged-out callers to /login without touching the API", async () => {
    await act(async () => {
      root.render(createElement(SaveProbe, { pinId: "pin-1" }));
    });
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(routerMocks.push).toHaveBeenCalledWith("/login");
    expect(apiMocks.saveFavorite).not.toHaveBeenCalled();
    expect(apiMocks.removeFavorite).not.toHaveBeenCalled();
  });

  it("saves optimistically on success", async () => {
    loggedIn();
    await act(async () => {
      root.render(createElement(SaveProbe, { pinId: "pin-1" }));
    });
    expect(container.textContent).toBe("unsaved:idle");
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(apiMocks.saveFavorite).toHaveBeenCalledWith("pin-1");
    expect(container.textContent).toBe("saved:idle");
  });

  it("rolls back to unsaved when the save API fails", async () => {
    loggedIn();
    apiMocks.saveFavorite.mockRejectedValue(new Error("network unavailable"));
    await act(async () => {
      root.render(createElement(SaveProbe, { pinId: "pin-1" }));
    });
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(container.textContent).toBe("unsaved:idle");
  });

  it("rolls back to saved when the unsave API fails", async () => {
    loggedIn();
    apiMocks.fetchFavoriteIDs.mockResolvedValue(["pin-1"]);
    apiMocks.removeFavorite.mockRejectedValue(new Error("network unavailable"));
    await act(async () => {
      root.render(createElement(SaveProbe, { pinId: "pin-1" }));
    });
    expect(container.textContent).toBe("saved:idle");
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(apiMocks.removeFavorite).toHaveBeenCalledWith("pin-1");
    expect(container.textContent).toBe("saved:idle");
  });

  it("sends one request for two clicks in the same tick", async () => {
    loggedIn();
    const pending = deferred<void>();
    apiMocks.saveFavorite.mockReturnValue(pending.promise);
    await act(async () => {
      root.render(createElement(SaveProbe, { pinId: "pin-1" }));
    });
    // Two synchronous clicks before React re-renders: the ref-based
    // in-flight guard drops the second even though `saving` is stale.
    const button = container.querySelector("button") as HTMLButtonElement;
    button.click();
    button.click();
    await act(async () => {
      pending.resolve();
      await pending.promise;
    });
    expect(apiMocks.saveFavorite).toHaveBeenCalledTimes(1);
    expect(container.textContent).toBe("saved:idle");
  });

  it("drops a re-entrant click while a save is in flight", async () => {
    loggedIn();
    const pending = deferred<void>();
    apiMocks.saveFavorite.mockReturnValue(pending.promise);
    await act(async () => {
      root.render(createElement(SaveProbe, { pinId: "pin-1" }));
    });
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    // The in-flight guard disables the button until the API settles, so a
    // second click while busy cannot re-fire the handler.
    expect((container.querySelector("button") as HTMLButtonElement).disabled).toBe(true);
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(apiMocks.saveFavorite).toHaveBeenCalledTimes(1);
    await act(async () => {
      pending.resolve();
      await pending.promise;
    });
    expect(container.textContent).toBe("saved:idle");
  });
});
