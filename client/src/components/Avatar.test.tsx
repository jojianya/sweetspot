// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import Avatar from "./Avatar";

describe("Avatar media resolution", () => {
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
  });

  it("normalizes an absolute upload URL to the same-origin path", async () => {
    await act(async () => {
      root.render(<Avatar src="http://192.168.1.5:8081/uploads/a.webp" username="alice" />);
    });
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img!.getAttribute("src")).toBe("/uploads/a.webp");
  });

  it("renders the initial fallback for a rejected URL", async () => {
    await act(async () => {
      root.render(<Avatar src="javascript:alert(1)" username="alice" />);
    });
    expect(container.querySelector("img")).toBeNull();
    expect(container.textContent).toContain("A");
  });
});
