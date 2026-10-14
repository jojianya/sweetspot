// @vitest-environment jsdom
import { act, createElement, useState } from "react";
import { createRoot, Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useClearOnChange } from "./useClearOnChange";

function Probe({ id, onClear }: { id: string; onClear: () => void }) {
  const [value, setValue] = useState(`stale-${id}`);
  useClearOnChange(id, () => {
    setValue("cleared");
    onClear();
  });
  return <span>{value}</span>;
}

describe("useClearOnChange", () => {
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

  it("does not clear on first render or stable key", async () => {
    const onClear = vi.fn();
    await act(async () => {
      root.render(createElement(Probe, { id: "a", onClear }));
    });
    expect(container.textContent).toBe("stale-a");
    expect(onClear).not.toHaveBeenCalled();
    await act(async () => {
      root.render(createElement(Probe, { id: "a", onClear }));
    });
    expect(onClear).not.toHaveBeenCalled();
  });

  it("clears during render when the key changes, before paint", async () => {
    const onClear = vi.fn();
    await act(async () => {
      root.render(createElement(Probe, { id: "a", onClear }));
    });
    await act(async () => {
      root.render(createElement(Probe, { id: "b", onClear }));
    });
    expect(onClear).toHaveBeenCalledTimes(1);
    expect(container.textContent).toBe("cleared");
  });
});
