// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import PinViews from "./PinViews";

describe("PinViews", () => {
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

  it.each([
    [0, "0 views"],
    [1, "1 view"],
    [2, "2 views"],
    [1234, "1,234 views"],
  ])("renders %d as %s in plain text", async (views, text) => {
    await act(async () => {
      root.render(createElement(PinViews, { views }));
    });
    expect(container.textContent).toBe(text);
    expect(container.querySelector("svg")).toBeNull();
  });
});
