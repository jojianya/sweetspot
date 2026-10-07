// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";
import { usePinView } from "./usePinView";

const apiMocks = vi.hoisted(() => ({
  registerPinView: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

function ViewProbe({ pinId, ownerId }: { pinId: string | null; ownerId?: string | null }) {
  usePinView(pinId, ownerId);
  return <span>{pinId ?? "none"}</span>;
}

function ResultProbe({ pinId, ownerId }: { pinId: string | null; ownerId?: string | null }) {
  const result = usePinView(pinId, ownerId);
  return <span>{result === null ? "none" : `${result.pinId}:${result.views}`}</span>;
}

describe("usePinView", () => {
  let container: HTMLDivElement;
  let root: Root;
  let debugSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    apiMocks.registerPinView.mockReset().mockResolvedValue(5);
    useAuth.setState({ user: null });
    useAuth.setState({
      user: { id: "viewer-1", username: "viewer", avatar_url: null, role: "user" },
    });
    debugSpy = vi.spyOn(console, "debug").mockImplementation(() => {});
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
    debugSpy.mockRestore();
  });

  it("registers once per pin id across re-renders", async () => {
    await act(async () => {
      root.render(createElement(ViewProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    await act(async () => {
      root.render(createElement(ViewProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    expect(apiMocks.registerPinView).toHaveBeenCalledTimes(1);
    expect(apiMocks.registerPinView).toHaveBeenCalledWith("pin-1");
  });

  it("registers again for a different pin", async () => {
    await act(async () => {
      root.render(createElement(ViewProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    await act(async () => {
      root.render(createElement(ViewProbe, { pinId: "pin-2", ownerId: "owner-9" }));
    });
    expect(apiMocks.registerPinView).toHaveBeenCalledTimes(2);
  });

  it("skips logged-out visitors", async () => {
    useAuth.setState({ user: null });
    await act(async () => {
      root.render(createElement(ViewProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    expect(apiMocks.registerPinView).not.toHaveBeenCalled();
  });

  it("skips the owner's own opens", async () => {
    await act(async () => {
      root.render(createElement(ViewProbe, { pinId: "pin-1", ownerId: "viewer-1" }));
    });
    expect(apiMocks.registerPinView).not.toHaveBeenCalled();
  });

  it("logs failures at debug level without surfacing", async () => {
    apiMocks.registerPinView.mockRejectedValue(new Error("offline"));
    await act(async () => {
      root.render(createElement(ViewProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    expect(debugSpy).toHaveBeenCalledTimes(1);
  });

  it("returns the counted views once", async () => {
    apiMocks.registerPinView.mockResolvedValue(7);
    await act(async () => {
      root.render(createElement(ResultProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    expect(container.textContent).toBe("pin-1:7");
  });

  it("returns null on skips and on the repeat guard", async () => {
    await act(async () => {
      root.render(createElement(ResultProbe, { pinId: "pin-1", ownerId: "viewer-1" }));
    });
    expect(container.textContent).toBe("none");
    await act(async () => {
      root.render(createElement(ResultProbe, { pinId: "pin-2", ownerId: "owner-9" }));
    });
    await act(async () => {
      root.render(createElement(ResultProbe, { pinId: "pin-2", ownerId: "owner-9" }));
    });
    expect(apiMocks.registerPinView).toHaveBeenCalledTimes(1);
  });

  it("clears the previous pin result while the next pin loads", async () => {
    apiMocks.registerPinView.mockReset();
    let resolveSecond!: (v: number) => void;
    apiMocks.registerPinView.mockImplementation((id: string) => {
      if (id === "pin-1") return Promise.resolve(7);
      return new Promise<number>((done) => {
        resolveSecond = done;
      });
    });
    await act(async () => {
      root.render(createElement(ResultProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    expect(container.textContent).toBe("pin-1:7");
    await act(async () => {
      root.render(createElement(ResultProbe, { pinId: "pin-2", ownerId: "owner-9" }));
    });
    expect(container.textContent).toBe("none");
    await act(async () => {
      resolveSecond(21);
    });
    expect(container.textContent).toBe("pin-2:21");
  });

  it("returns null when the call fails", async () => {
    apiMocks.registerPinView.mockRejectedValue(new Error("offline"));
    await act(async () => {
      root.render(createElement(ResultProbe, { pinId: "pin-1", ownerId: "owner-9" }));
    });
    expect(container.textContent).toBe("none");
    expect(debugSpy).toHaveBeenCalledTimes(1);
  });
});

describe("usePinView stale results", () => {
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

  it("exposes A's response only to the instance that asked, never to B", async () => {
    const first = { resolve: null as null | ((v: number) => void) };
    const second = { resolve: null as null | ((v: number) => void) };
    apiMocks.registerPinView.mockImplementation((id: string) => {
      if (id === "pin-a") return new Promise((done) => { first.resolve = done; });
      return new Promise((done) => { second.resolve = done; });
    });

    function Show({ id }: { id: string }) {
      const r = usePinView(id, "owner-9");
      return <span>{r === null ? "none" : `${r.pinId}:${r.views}`}</span>;
    }

    await act(async () => {
      root.render(createElement(Show, { id: "pin-a" }));
    });
    await act(async () => {
      root.unmount();
    });

    const container2 = document.createElement("div");
    document.body.appendChild(container2);
    const root2 = createRoot(container2);
    await act(async () => {
      root2.render(createElement(Show, { id: "pin-b" }));
    });

    await act(async () => {
      first.resolve?.(99);
    });
    expect(container2.textContent).toBe("none");

    await act(async () => {
      second.resolve?.(21);
    });
    expect(container2.textContent).toBe("pin-b:21");
    await act(async () => {
      root2.unmount();
    });
    container2.remove();
  });
});
