// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";
import { useReaction } from "./useReaction";

const apiMocks = vi.hoisted(() => ({
  reactToPin: vi.fn(),
  unreactToPin: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

const routerMocks = vi.hoisted(() => ({
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: routerMocks.push }),
}));

const PIN = { id: "pin-1", reactedByMe: false, goodSpotCount: 0 };

function ReactionProbe({
  pin = PIN,
}: {
  pin?: { id: string; reactedByMe?: boolean; goodSpotCount: number };
}) {
  const { reacted, count, busy, error, toggle } = useReaction(pin);
  return (
    <button type="button" disabled={busy} onClick={toggle}>
      {reacted ? "reacted" : "not"}:{count}:{busy ? "busy" : "idle"}
      {error ? `:${error}` : ""}
    </button>
  );
}

function loggedIn() {
  useAuth.setState({
    user: { id: "user-1", username: "someone", avatar_url: null, role: "user" },
  });
}

describe("useReaction", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.reactToPin.mockReset().mockResolvedValue({ reacted: true, good_spot_count: 1 });
    apiMocks.unreactToPin.mockReset().mockResolvedValue({ reacted: false, good_spot_count: 0 });
    routerMocks.push.mockReset();
    useAuth.setState({ user: null });
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
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

  it("seeds state from the pin's reacted_by_me and good_spot_count", async () => {
    loggedIn();
    await act(async () => {
      root.render(createElement(ReactionProbe, {
        pin: { id: "pin-9", reactedByMe: true, goodSpotCount: 7 },
      }));
    });
    // No request: the viewer's own state arrives with the pin.
    expect(container.textContent).toBe("reacted:7:idle");
    expect(apiMocks.reactToPin).not.toHaveBeenCalled();
    expect(apiMocks.unreactToPin).not.toHaveBeenCalled();
  });

  it("defaults to not-reacted and zero when the pin carries no viewer state", async () => {
    loggedIn();
    await act(async () => {
      root.render(createElement(ReactionProbe, { pin: { id: "pin-1", goodSpotCount: 0 } }));
    });
    expect(container.textContent).toBe("not:0:idle");
  });

  it("sends logged-out callers to /login without touching the API", async () => {
    await act(async () => {
      root.render(createElement(ReactionProbe));
    });
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(routerMocks.push).toHaveBeenCalledWith("/login");
    expect(apiMocks.reactToPin).not.toHaveBeenCalled();
    expect(apiMocks.unreactToPin).not.toHaveBeenCalled();
  });

  it("flips to reacted with an optimistic count, then takes the server's count", async () => {
    loggedIn();
    // The server reports a different total than the optimistic guess: another
    // account reacted at the same moment, so 1 + 1 = 3 rather than 1.
    apiMocks.reactToPin.mockResolvedValue({ reacted: true, good_spot_count: 3 });
    await act(async () => {
      root.render(createElement(ReactionProbe, { pin: { id: "pin-1", goodSpotCount: 2 } }));
    });
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(apiMocks.reactToPin).toHaveBeenCalledWith("pin-1");
    // The optimistic guess is visible immediately.
    expect(container.textContent).toBe("reacted:3:idle");
  });

  it("rolls back and shows a reason when the react call fails", async () => {
    loggedIn();
    apiMocks.reactToPin.mockRejectedValue(new Error("network unavailable"));
    await act(async () => {
      root.render(createElement(ReactionProbe, { pin: { id: "pin-1", goodSpotCount: 4 } }));
    });
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    // Rolled back to the pre-tap state, count included, with the error surfaced.
    expect(container.textContent).toBe("not:4:idle:network unavailable");
  });

  it("rolls back when the unreact call fails", async () => {
    loggedIn();
    apiMocks.unreactToPin.mockRejectedValue(new Error("network unavailable"));
    await act(async () => {
      root.render(
        createElement(ReactionProbe, { pin: { id: "pin-1", reactedByMe: true, goodSpotCount: 1 } })
      );
    });
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(apiMocks.unreactToPin).toHaveBeenCalledWith("pin-1");
    expect(container.textContent).toBe("reacted:1:idle:network unavailable");
  });

  it("drops a second click while the first is in flight", async () => {
    loggedIn();
    let resolve!: (v: unknown) => void;
    apiMocks.reactToPin.mockReturnValue(new Promise((r) => (resolve = r)));
    await act(async () => {
      root.render(createElement(ReactionProbe, { pin: { id: "pin-1", goodSpotCount: 0 } }));
    });
    const button = container.querySelector("button") as HTMLButtonElement;
    await act(async () => {
      button.click();
      button.click();
      button.click();
    });
    expect(apiMocks.reactToPin).toHaveBeenCalledTimes(1);
    // Disabled while in flight, so the control cannot be re-triggered.
    expect(button.disabled).toBe(true);
    await act(async () => {
      resolve({ reacted: true, good_spot_count: 1 });
    });
    expect(container.textContent).toBe("reacted:1:idle");
  });
});
