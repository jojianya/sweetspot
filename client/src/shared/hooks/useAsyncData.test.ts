// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAsyncData } from "./useAsyncData";

const EMPTY_DEPS: unknown[] = [];

type Fetcher = (signal: AbortSignal) => Promise<string>;

function TestHook({ fetcher, value }: { fetcher: Fetcher; value: number }) {
  useAsyncData(fetcher, EMPTY_DEPS, { enabled: true });
  return createElement("span", null, value);
}

describe("useAsyncData", () => {
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

  it("fetches once and does not refetch when empty deps stay stable on rerender", async () => {
    const fetcher = vi.fn(() => new Promise<string>(() => {}));

    await act(async () => {
      root.render(createElement(TestHook, { fetcher, value: 0 }));
    });
    expect(fetcher).toHaveBeenCalledTimes(1);

    await act(async () => {
      root.render(createElement(TestHook, { fetcher, value: 1 }));
    });
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
});
