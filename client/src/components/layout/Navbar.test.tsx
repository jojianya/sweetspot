// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import Navbar from "./Navbar";
import { useAuth } from "@/store/auth";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/lib/api", () => ({
  logout: vi.fn(),
}));

vi.mock("@/lib/api/users", () => ({
  fetchMe: vi.fn().mockRejectedValue(new Error("offline")),
}));

function navExists(container: HTMLElement): boolean {
  return container.querySelector("nav") !== null;
}

function linkExists(container: HTMLElement, text: string): boolean {
  return (
    Array.from(container.querySelectorAll("a")).some((a) => a.textContent === text)
  );
}

describe("Navbar account navigation", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
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

  async function renderNavbar(props?: { hideAccountNav?: boolean }) {
    await act(async () => {
      root.render(<Navbar backHref="/" backLabel="Back to map" {...props} />);
    });
  }

  it("shows role links for an admin by default", async () => {
    useAuth.setState({
      user: { id: "u1", username: "mod", avatar_url: null, role: "admin" },
    });
    await renderNavbar();
    expect(navExists(container)).toBe(true);
    expect(linkExists(container, "Reports")).toBe(true);
    expect(linkExists(container, "Log in")).toBe(false);
  });

  it("shows login links when logged out by default", async () => {
    useAuth.setState({ user: null });
    await renderNavbar();
    expect(navExists(container)).toBe(true);
    expect(linkExists(container, "Log in")).toBe(true);
  });

  it("hides the whole nav with hideAccountNav when logged in", async () => {
    useAuth.setState({
      user: { id: "u1", username: "mod", avatar_url: null, role: "admin" },
    });
    await renderNavbar({ hideAccountNav: true });
    expect(navExists(container)).toBe(false);
    expect(linkExists(container, "Reports")).toBe(false);
  });

  it("hides the whole nav with hideAccountNav when logged out", async () => {
    useAuth.setState({ user: null });
    await renderNavbar({ hideAccountNav: true });
    expect(navExists(container)).toBe(false);
    expect(linkExists(container, "Log in")).toBe(false);
  });

  it("keeps the back link visible with hideAccountNav", async () => {
    useAuth.setState({ user: null });
    await renderNavbar({ hideAccountNav: true });
    expect(linkExists(container, "Back to map")).toBe(true);
  });
});
