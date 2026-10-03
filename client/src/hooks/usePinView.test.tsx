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
});
