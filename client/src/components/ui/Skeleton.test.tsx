// @vitest-environment jsdom
import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Skeleton, SkeletonRegion } from "@/components/ui/Skeleton";

describe("Skeleton", () => {
  let container: HTMLDivElement;
  let root: ReturnType<typeof createRoot>;

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

  it("renders with aria-hidden and pulse animation class", async () => {
    await act(async () => {
      root.render(<Skeleton className="h-4 w-20" data-testid="skeleton" />);
    });
    const el = container.querySelector("[data-testid=skeleton]");
    expect(el).not.toBeNull();
    expect(el!.getAttribute("aria-hidden")).toBe("true");
    expect(el!.classList.contains("motion-safe:animate-pulse")).toBe(true);
    expect(el!.classList.contains("bg-zinc-200")).toBe(true);
    expect(el!.classList.contains("dark:bg-zinc-700")).toBe(true);
  });

  it("forwards ref", async () => {
    const ref = { current: null as HTMLDivElement | null };
    await act(async () => {
      root.render(<Skeleton ref={ref} data-testid="skeleton-ref" />);
    });
    expect(ref.current).not.toBeNull();
    expect(ref.current).toBe(container.querySelector("[data-testid=skeleton-ref]"));
  });
});

describe("SkeletonRegion", () => {
  let container: HTMLDivElement;
  let root: ReturnType<typeof createRoot>;

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

  it("renders with role=status and aria-busy", async () => {
    await act(async () => {
      root.render(
        <SkeletonRegion label="Loading pins…">
          <div data-testid="child" />
        </SkeletonRegion>
      );
    });
    const region = container.querySelector("[role=status]");
    expect(region).not.toBeNull();
    expect(region!.getAttribute("aria-busy")).toBe("true");
    expect(region!.getAttribute("aria-label")).toBe("Loading pins…");
    const srOnly = container.querySelector(".sr-only");
    expect(srOnly).not.toBeNull();
    expect(srOnly!.textContent).toBe("Loading pins…");
    expect(container.querySelector("[data-testid=child]")).not.toBeNull();
  });

  it("uses default label when none provided", async () => {
    await act(async () => {
      root.render(
        <SkeletonRegion>
          <div data-testid="child" />
        </SkeletonRegion>
      );
    });
    const region = container.querySelector("[role=status]");
    expect(region).not.toBeNull();
    expect(region!.getAttribute("aria-label")).toBe("Loading…");
  });
});