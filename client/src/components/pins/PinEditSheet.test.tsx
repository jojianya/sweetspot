// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import type { PinDetail } from "@/lib/types";
import PinEditSheet from "./PinEditSheet";

const apiMocks = vi.hoisted(() => ({
  fetchPin: vi.fn(),
  updatePin: vi.fn(),
  deletePin: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

const pin: PinDetail = {
  id: "pin-1",
  user_id: "user-1",
  location: "POINT(0 0)",
  geohash: "s000",
  caption: "A great spot",
  category_id: 1,
  is_hidden: false,
  views: 4,
  created_at: "2026-01-02T00:00:00Z",
  category: "Food",
  username: "alice",
  avatar_url: null,
  photos: [],
};

function buttonByName(container: HTMLElement, name: string): HTMLButtonElement | null {
  const buttons = Array.from(container.querySelectorAll("button"));
  return (buttons.find((b) => b.textContent === name) as HTMLButtonElement) ?? null;
}

describe("PinEditSheet update", () => {
  let container: HTMLDivElement;
  let root: Root;
  let onClose: Mock<() => void>;
  let onUpdated: Mock<(pin: PinDetail) => void>;

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    onClose = vi.fn<() => void>();
    onUpdated = vi.fn<(pin: PinDetail) => void>();
    apiMocks.fetchPin.mockReset();
    apiMocks.updatePin.mockReset();
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  async function renderSheet() {
    await act(async () => {
      root.render(
        <PinEditSheet
          pin={pin}
          categories={[{ id: 1, name: "Food", slug: "food" }]}
          onClose={onClose}
          onUpdated={onUpdated}
          onDeleted={vi.fn()}
        />
      );
    });
  }

  async function clickButton(name: string) {
    await act(async () => {
      buttonByName(container, name)!.click();
    });
  }

  function updatedPhoto(overrides: Partial<PinDetail["photos"][number]> = {}): PinDetail["photos"][number] {
    return {
      id: "photo-2",
      pin_id: "pin-1",
      photo_url: "http://api.test/new.webp",
      thumbnail_url: "http://api.test/new-thumb.webp",
      position: 0,
      created_at: "2026-02-01T00:00:00Z",
      ...overrides,
    };
  }

  // The update response carries the pin's photo set, so the sheet must adopt it
  // instead of refetching the pin it was just handed.
  it("adopts the photo set from the update response without refetching", async () => {
    const photo = updatedPhoto();
    apiMocks.updatePin.mockResolvedValue({ pin: { ...pin, caption: "Renamed" }, photos: [photo] });
    await renderSheet();

    await clickButton("Save changes");
    await act(async () => {});

    expect(apiMocks.updatePin).toHaveBeenCalledTimes(1);
    expect(apiMocks.fetchPin).not.toHaveBeenCalled();
    expect(onUpdated).toHaveBeenCalledTimes(1);
    const passed = onUpdated.mock.calls[0][0];
    expect(passed.caption).toBe("Renamed");
    expect(passed.photos).toEqual([photo]);
    // Fields the edit cannot change carry over from the sheet's own pin.
    expect(passed.category).toBe("Food");
    expect(onClose).toHaveBeenCalled();
  });
});

describe("PinEditSheet delete", () => {
  let container: HTMLDivElement;
  let root: Root;
  let onClose: () => void;
  let onUpdated: (pin: PinDetail) => void;
  let onDeleted: (id: string) => void;

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    onClose = vi.fn();
    onUpdated = vi.fn();
    onDeleted = vi.fn();
    apiMocks.deletePin.mockReset();
    apiMocks.deletePin.mockResolvedValue(undefined);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  async function renderSheet() {
    await act(async () => {
      root.render(
        <PinEditSheet
          pin={pin}
          categories={[{ id: 1, name: "Food", slug: "food" }]}
          onClose={onClose}
          onUpdated={onUpdated}
          onDeleted={onDeleted}
        />
      );
    });
  }

  it("shows a Delete pin action", async () => {
    await renderSheet();
    expect(buttonByName(container, "Delete pin")).not.toBeNull();
  });

  it("asks for confirmation, focuses it, and deletes on confirm", async () => {
    await renderSheet();
    await act(async () => {
      buttonByName(container, "Delete pin")!.click();
    });
    const confirmRegion = container.querySelector('[aria-label="Confirm pin deletion"]');
    expect(confirmRegion).not.toBeNull();
    const confirm = buttonByName(container, "Yes, delete pin");
    expect(confirm).not.toBeNull();
    expect(document.activeElement).toBe(confirm);
    expect(buttonByName(container, "Keep pin")).not.toBeNull();

    await act(async () => {
      confirm!.click();
    });
    expect(apiMocks.deletePin).toHaveBeenCalledWith("pin-1");
    expect(onDeleted).toHaveBeenCalledWith("pin-1");
    expect(onClose).toHaveBeenCalled();
  });

  it("cancelling keeps the pin and closes nothing", async () => {
    await renderSheet();
    await act(async () => {
      buttonByName(container, "Delete pin")!.click();
    });
    await act(async () => {
      buttonByName(container, "Keep pin")!.click();
    });
    expect(apiMocks.deletePin).not.toHaveBeenCalled();
    expect(onDeleted).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(buttonByName(container, "Delete pin")).not.toBeNull();
  });

  it("shows a friendly error when deletion fails", async () => {
    apiMocks.deletePin.mockRejectedValue(new Error("you can only delete your own pins"));
    await renderSheet();
    await act(async () => {
      buttonByName(container, "Delete pin")!.click();
    });
    await act(async () => {
      buttonByName(container, "Yes, delete pin")!.click();
    });
    const alert = container.querySelector('[role="alert"]');
    expect(alert).not.toBeNull();
    expect(alert!.textContent).toContain("you can only delete your own pins");
    expect(onDeleted).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
