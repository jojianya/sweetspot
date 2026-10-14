// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePinShare } from "./usePinShare";

function ShareProbe({
  caption,
  pinId,
  point,
}: {
  caption: string | null;
  pinId: string;
  point: { lat: number; lng: number } | null;
}) {
  const { copied, handleShare, goDirections } = usePinShare(caption, pinId, point);
  return (
    <>
      <button type="button" data-testid="share" onClick={() => void handleShare()}>
        {copied ? "copied" : "share"}
      </button>
      <button type="button" data-testid="directions" onClick={goDirections}>
        directions
      </button>
    </>
  );
}

describe("usePinShare", () => {
  let container: HTMLDivElement;
  let root: Root;
  let openSpy: ReturnType<typeof vi.spyOn>;

  const shareText = (caption: string | null) =>
    `Check out ${caption ?? "this place"} on GoodSpot`;
  const pinUrl = (id: string) => `${window.location.origin}/pin/${id}`;

  beforeEach(() => {
    vi.useRealTimers();
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    openSpy = vi.spyOn(window, "open").mockReturnValue(null);
    Object.defineProperty(navigator, "share", { value: undefined, configurable: true });
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
    });
    vi.spyOn(window, "setTimeout");
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    openSpy.mockRestore();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  function shareButton(): HTMLButtonElement {
    return container.querySelector('[data-testid="share"]') as HTMLButtonElement;
  }

  function directionsButton(): HTMLButtonElement {
    return container.querySelector('[data-testid="directions"]') as HTMLButtonElement;
  }

  function clipboardWrite(): ReturnType<typeof vi.fn> {
    return (navigator.clipboard as unknown as { writeText: ReturnType<typeof vi.fn> })
      .writeText;
  }

  it("prefers the native share sheet and skips the clipboard", async () => {
    const share = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "share", { value: share, configurable: true });
    await act(async () => {
      root.render(createElement(ShareProbe, { caption: "A spot", pinId: "pin-1", point: null }));
    });
    await act(async () => {
      shareButton().click();
    });
    expect(share).toHaveBeenCalledWith({
      title: shareText("A spot"),
      text: shareText("A spot"),
      url: pinUrl("pin-1"),
    });
    expect(clipboardWrite()).not.toHaveBeenCalled();
    expect(container.textContent).toContain("share");
  });

  it("falls back to null-caption text plus clipboard when native share is absent", async () => {
    await act(async () => {
      root.render(createElement(ShareProbe, { caption: null, pinId: "pin-9", point: null }));
    });
    await act(async () => {
      shareButton().click();
    });
    expect(clipboardWrite()).toHaveBeenCalledWith(`${shareText(null)} ${pinUrl("pin-9")}`);
    expect(container.textContent).toContain("copied");
  });

  it("falls back to clipboard when the native sheet is dismissed", async () => {
    const share = vi.fn().mockRejectedValue(new Error("dismissed"));
    Object.defineProperty(navigator, "share", { value: share, configurable: true });
    await act(async () => {
      root.render(createElement(ShareProbe, { caption: "A spot", pinId: "pin-1", point: null }));
    });
    await act(async () => {
      shareButton().click();
    });
    expect(clipboardWrite()).toHaveBeenCalledWith(`${shareText("A spot")} ${pinUrl("pin-1")}`);
    expect(container.textContent).toContain("copied");
  });

  it("swallows clipboard failures without surfacing an error", async () => {
    clipboardWrite().mockRejectedValue(new Error("denied"));
    await act(async () => {
      root.render(createElement(ShareProbe, { caption: "A spot", pinId: "pin-1", point: null }));
    });
    await act(async () => {
      shareButton().click();
    });
    expect(container.textContent).toContain("share");
  });

  it("opens Google Maps directions for the point and ignores a null point", async () => {
    await act(async () => {
      root.render(
        createElement(ShareProbe, { caption: "A spot", pinId: "pin-1", point: { lat: 1.5, lng: 2.5 } })
      );
    });
    await act(async () => {
      directionsButton().click();
    });
    expect(openSpy).toHaveBeenCalledWith(
      "https://www.google.com/maps/dir/?api=1&destination=1.5,2.5",
      "_blank",
      "noopener,noreferrer"
    );

    await act(async () => {
      root.render(
        createElement(ShareProbe, { caption: "A spot", pinId: "pin-1", point: null })
      );
    });
    openSpy.mockClear();
    await act(async () => {
      directionsButton().click();
    });
    expect(openSpy).not.toHaveBeenCalled();
  });
});
