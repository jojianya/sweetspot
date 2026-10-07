// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import LocateButton from "./LocateButton";

const geoMocks = vi.hoisted(() => ({
  getCurrentPosition: vi.fn(),
  toGeoCoords: vi.fn(() => ({ lat: 1, lng: 2 })),
}));

vi.mock("@/lib/utils", () => ({
  geolocationAvailable: () => true,
  getCurrentPosition: geoMocks.getCurrentPosition,
  toGeoCoords: geoMocks.toGeoCoords,
}));

describe("LocateButton", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    geoMocks.getCurrentPosition.mockReset();
    geoMocks.toGeoCoords.mockClear();
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  async function renderButton(onLocate: () => void = vi.fn()): Promise<void> {
    await act(async () => {
      root.render(createElement(LocateButton, { onLocate }));
    });
  }

  function button(): HTMLButtonElement {
    return container.querySelector("button")!;
  }

  it("labels the icon-only button", async () => {
    geoMocks.getCurrentPosition.mockResolvedValue({ coords: {} });
    await renderButton();
    expect(button().getAttribute("aria-label")).toBe("Go to my location");
  });

  it("ignores the position result after unmount", async () => {
    let resolve!: (pos: unknown) => void;
    geoMocks.getCurrentPosition.mockReturnValue(
      new Promise((done) => {
        resolve = done;
      })
    );
    const onLocate = vi.fn();
    await renderButton(onLocate);
    await act(async () => {
      button().dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    await act(async () => {
      root.unmount();
    });
    await act(async () => {
      resolve({ coords: {} });
    });
    expect(onLocate).not.toHaveBeenCalled();
  });

  it("announces failures to assistive technology", async () => {
    geoMocks.getCurrentPosition.mockRejectedValue(new Error("denied"));
    await renderButton();
    await act(async () => {
      button().dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    const alert = container.querySelector('[role="alert"], [role="status"]');
    expect(alert).not.toBeNull();
    expect(alert!.textContent).toMatch(/unavailable|denied|location/i);
  });
});
