// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";
import GoodSpotButton from "./GoodSpotButton";

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

function loggedIn() {
  useAuth.setState({
    user: { id: "user-1", username: "someone", avatar_url: null, role: "user" },
  });
}

function button(container: HTMLElement): HTMLButtonElement | null {
  return container.querySelector("button");
}

describe("GoodSpotButton", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.reactToPin.mockReset().mockResolvedValue({ reacted: true, good_spot_count: 1 });
    apiMocks.unreactToPin.mockReset().mockResolvedValue({ reacted: false, good_spot_count: 0 });
    routerMocks.push.mockReset();
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

  it("seeds aria-pressed from reacted_by_me and names the count in the label", async () => {
    loggedIn();
    await act(async () => {
      root.render(
        createElement(GoodSpotButton, {
          pin: { id: "pin-1", reactedByMe: true, goodSpotCount: 12 },
          isOwnPin: false,
        })
      );
    });
    const el = button(container);
    expect(el).not.toBeNull();
    expect(el?.getAttribute("aria-pressed")).toBe("true");
    // The count is in the accessible name, not only in a visual label.
    expect(el?.getAttribute("aria-label")).toBe("Good spot, 12");
    expect(container.textContent).toContain("Good spot");
  });

  it("toggles aria-pressed on a successful tap", async () => {
    loggedIn();
    await act(async () => {
      root.render(
        createElement(GoodSpotButton, {
          pin: { id: "pin-1", goodSpotCount: 0 },
          isOwnPin: false,
        })
      );
    });
    expect(button(container)?.getAttribute("aria-pressed")).toBe("false");
    await act(async () => {
      button(container)?.click();
    });
    expect(apiMocks.reactToPin).toHaveBeenCalledWith("pin-1");
    expect(button(container)?.getAttribute("aria-pressed")).toBe("true");
    expect(button(container)?.getAttribute("aria-label")).toBe("Good spot, 1");
  });

  it("renders no button at all for the author's own pin, only the count", async () => {
    loggedIn();
    await act(async () => {
      root.render(
        createElement(GoodSpotButton, {
          pin: { id: "pin-1", goodSpotCount: 3 },
          isOwnPin: true,
        })
      );
    });
    // Plain text, not a disabled control: the action does not exist here.
    expect(button(container)).toBeNull();
    expect(container.textContent).toContain("3 good spots");
  });

  it("surfaces a failure inside a role=alert", async () => {
    loggedIn();
    apiMocks.reactToPin.mockRejectedValue(new Error("too many requests"));
    await act(async () => {
      root.render(
        createElement(GoodSpotButton, {
          pin: { id: "pin-1", goodSpotCount: 0 },
          isOwnPin: false,
        })
      );
    });
    await act(async () => {
      button(container)?.click();
    });
    const alert = container.querySelector("[role='alert']");
    expect(alert).not.toBeNull();
    expect(alert?.textContent).toBe("too many requests");
    // And the toggle rolled back, so the control does not lie about the state.
    expect(button(container)?.getAttribute("aria-pressed")).toBe("false");
  });

  it("disables the button while a call is in flight", async () => {
    loggedIn();
    let resolve!: (v: unknown) => void;
    apiMocks.reactToPin.mockReturnValue(new Promise((r) => (resolve = r)));
    await act(async () => {
      root.render(
        createElement(GoodSpotButton, {
          pin: { id: "pin-1", goodSpotCount: 0 },
          isOwnPin: false,
        })
      );
    });
    await act(async () => {
      button(container)?.click();
    });
    expect(button(container)?.disabled).toBe(true);
    await act(async () => {
      resolve({ reacted: true, good_spot_count: 1 });
    });
    expect(button(container)?.disabled).toBe(false);
  });

  it("sends a logged-out tap to /login without calling the API", async () => {
    await act(async () => {
      root.render(
        createElement(GoodSpotButton, {
          pin: { id: "pin-1", goodSpotCount: 0 },
          isOwnPin: false,
        })
      );
    });
    await act(async () => {
      button(container)?.click();
    });
    expect(routerMocks.push).toHaveBeenCalledWith("/login");
    expect(apiMocks.reactToPin).not.toHaveBeenCalled();
  });
});
