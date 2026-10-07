// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";
import MapNavBar from "./MapNavBar";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("@/lib/api/users", () => ({
  fetchMe: vi.fn().mockRejectedValue(new Error("offline")),
}));
vi.mock("./ThemeToggle", () => ({
  default: () => (
    <button type="button" role="menuitem">
      Theme
    </button>
  ),
}));

const baseProps = {
  center: { lat: 0, lng: 0 },
  onSelectPlace: vi.fn(),
  onSelectPin: vi.fn(),
  categories: [],
  selectedCategory: null,
  onSelectCategory: vi.fn(),
  onOpenSaved: vi.fn(),
};

describe("MapNavBar account menu", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    useAuth.setState({
      user: { id: "u-1", username: "alice", avatar_url: null, role: "user" },
    } as never);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null } as never);
    vi.clearAllMocks();
  });

  async function openMenu(): Promise<HTMLButtonElement> {
    await act(async () => {
      root.render(createElement(MapNavBar, baseProps));
    });
    const trigger = container.querySelector('button[aria-label="Account"]') as HTMLButtonElement;
    await act(async () => {
      trigger.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    return trigger;
  }

  function menu(): HTMLElement | null {
    return container.querySelector('[role="menu"]');
  }

  it("declares a menu trigger", async () => {
    const trigger = await openMenu();
    expect(trigger.getAttribute("aria-haspopup")).toBe("menu");
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(menu()).not.toBeNull();
  });

  it("closes on Escape and returns focus to the trigger", async () => {
    const trigger = await openMenu();
    expect(menu()).not.toBeNull();
    await act(async () => {
      document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    });
    expect(menu()).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("closes on outside click", async () => {
    await openMenu();
    expect(menu()).not.toBeNull();
    await act(async () => {
      document.body.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    });
    expect(menu()).toBeNull();
  });

  it("moves focus between items with arrow keys", async () => {
    await openMenu();
    const items = Array.from(menu()!.querySelectorAll('[role="menuitem"]')) as HTMLElement[];
    expect(items.length).toBeGreaterThan(1);
    await act(async () => {
      items[0].focus();
      items[0].dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
    });
    expect(document.activeElement).toBe(items[1]);
    await act(async () => {
      (document.activeElement as HTMLElement).dispatchEvent(
        new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true })
      );
    });
    expect(document.activeElement).toBe(items[0]);
  });
});
