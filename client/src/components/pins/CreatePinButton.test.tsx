// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";
import CreatePinButton from "./CreatePinButton";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

const baseProps = {
  lat: 14.5,
  lng: 120.9,
  categories: [{ id: 1, name: "Food", slug: "food" }],
  posting: false,
  open: true,
  onOpenChange: vi.fn(),
  onCreated: vi.fn(),
  onSetLocation: vi.fn(),
};

describe("CreatePinButton labels", () => {
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
  });

  it("names the photo, category and caption fields", async () => {
    await act(async () => {
      root.render(createElement(CreatePinButton, baseProps));
    });
    const file = container.querySelector('input[type="file"]') as HTMLInputElement;
    const select = container.querySelector("select") as HTMLSelectElement;
    const caption = container.querySelector('input[type="text"]') as HTMLInputElement;
    expect(file?.getAttribute("aria-label")).toBeTruthy();
    expect(select?.getAttribute("aria-label")).toBeTruthy();
    expect(caption?.getAttribute("aria-label")).toBeTruthy();
  });
});
