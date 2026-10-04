// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePinStream } from "./usePinStream";

const VALID_PIN_ID = "11111111-1111-1111-1111-111111111111";

type Listener = (raw: { data: string }) => void;

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners = new Map<string, Listener[]>();
  onerror: ((ev: unknown) => void) | null = null;
  onopen: ((ev: unknown) => void) | null = null;
  closed = false;
  url: string;
  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }
  addEventListener(type: string, fn: Listener) {
    const list = this.listeners.get(type) ?? [];
    list.push(fn);
    this.listeners.set(type, list);
  }
  close() {
    this.closed = true;
  }
  fire(type: string, payload: unknown) {
    for (const fn of this.listeners.get(type) ?? []) fn({ data: JSON.stringify(payload) });
  }
  fail() {
    this.onerror?.({});
  }
  open() {
    this.onopen?.({});
  }
}

const apiMocks = vi.hoisted(() => ({ openPinStream: vi.fn() }));

vi.mock("@/lib/api", () => apiMocks);

function StreamProbe({
  bbox,
  onPin,
  onPinRemoved,
  onReconnect,
}: {
  bbox: string | null;
  onPin: (pin: { id: string }) => void;
  onPinRemoved?: (id: string) => void;
  onReconnect?: () => void;
}) {
  usePinStream(bbox, null, onPin as never, onPinRemoved, onReconnect);
  return null;
}

describe("usePinStream pin_removed", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    FakeEventSource.instances = [];
    apiMocks.openPinStream.mockReset().mockImplementation(() => {
      // Mirror the real openPinStream: one EventSource carrying both listeners.
      const es = new FakeEventSource("http://test/events") as unknown as EventSource;
      return es;
    });
    // Route the onPin/onPinRemoved callbacks through the fake like realtime.ts does.
    apiMocks.openPinStream.mockImplementation(
      (_bbox: string, _cat: number | null, onPin: Listener, onRemoved?: (ev: { id: string }) => void) => {
        const es = new FakeEventSource("http://test/events");
        (es as unknown as { __onPin: unknown }).__onPin = onPin;
        (es as unknown as { __onRemoved: unknown }).__onRemoved = onRemoved;
        es.addEventListener("pin", (raw) => {
          try {
            const data = JSON.parse((raw as { data: string }).data);
            onPin(data);
          } catch {
            /* ignore in test double */
          }
        });
        if (onRemoved) {
          es.addEventListener("pin_removed", (raw) => {
            const data = JSON.parse((raw as { data: string }).data);
            onRemoved(data);
          });
        }
        return es as unknown as EventSource;
      }
    );
    vi.useFakeTimers();
    const actEnv = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean };
    actEnv.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    vi.useRealTimers();
  });

  it("case 1: normal pin_removed delivered — calls onPinRemoved with the pin id", async () => {
    const onPin = vi.fn();
    const onRemoved = vi.fn();
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "1,2,3,4", onPin, onPinRemoved: onRemoved }));
    });
    const es = FakeEventSource.instances[0];
    await act(async () => {
      es.fire("pin_removed", { id: VALID_PIN_ID, location: "POINT(0 0)" });
    });
    expect(onRemoved).toHaveBeenCalledTimes(1);
    expect(onRemoved).toHaveBeenCalledWith(VALID_PIN_ID);
  });

  it("case 2: pin_removed never calls onPin — removals cannot produce pin events", async () => {
    const onPin = vi.fn();
    const onRemoved = vi.fn();
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "1,2,3,4", onPin, onPinRemoved: onRemoved }));
    });
    const es = FakeEventSource.instances[0];
    await act(async () => {
      es.fire("pin_removed", { id: VALID_PIN_ID, location: "POINT(0 0)" });
    });
    expect(onRemoved).toHaveBeenCalledTimes(1);
    expect(onPin).not.toHaveBeenCalled();
  });

  it("case 3: error alone never refetches — waits for the stream to be live again", async () => {
    const onPin = vi.fn();
    const onReconnect = vi.fn();
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "1,2,3,4", onPin, onReconnect }));
    });
    const es = FakeEventSource.instances[0];
    await act(async () => {
      es.fail();
    });
    expect(onReconnect).not.toHaveBeenCalled();
    // No timer refetch: time passing while the stream is still broken does nothing.
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(onReconnect).not.toHaveBeenCalled();
  });

  it("case 4: clean intentional re-subscription does not refetch — bbox change without error", async () => {
    const onPin = vi.fn();
    const onReconnect = vi.fn();
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "1,2,3,4", onPin, onReconnect }));
    });
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(FakeEventSource.instances[0].closed).toBe(false);
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "5,6,7,8", onPin, onReconnect }));
    });
    // Old stream closed, new stream opened, no error flag -> no refetch.
    expect(FakeEventSource.instances).toHaveLength(2);
    expect(FakeEventSource.instances[0].closed).toBe(true);
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(onReconnect).not.toHaveBeenCalled();
  });

  it("case 5: error then open refetches exactly once — successful reconnect ends the episode", async () => {
    const onPin = vi.fn();
    const onReconnect = vi.fn();
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "1,2,3,4", onPin, onReconnect }));
    });
    const es = FakeEventSource.instances[0];
    await act(async () => {
      es.fail();
    });
    expect(onReconnect).not.toHaveBeenCalled();
    // The browser's reconnect succeeds: refetch now that the stream is live again.
    await act(async () => {
      es.open();
    });
    expect(onReconnect).toHaveBeenCalledTimes(1);
    // A second open with no new error is a clean open — no further refetch.
    await act(async () => {
      es.open();
    });
    expect(onReconnect).toHaveBeenCalledTimes(1);
  });

  it("case 6: error, time passes, then open refetches on open", async () => {
    const onPin = vi.fn();
    const onReconnect = vi.fn();
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "1,2,3,4", onPin, onReconnect }));
    });
    const es = FakeEventSource.instances[0];
    await act(async () => {
      es.fail();
    });
    expect(onReconnect).not.toHaveBeenCalled();
    // Time passing while broken does not refetch.
    await act(async () => {
      vi.advanceTimersByTime(2000);
    });
    expect(onReconnect).not.toHaveBeenCalled();
    // The refetch happens on open, once the stream is live again.
    await act(async () => {
      es.open();
    });
    expect(onReconnect).toHaveBeenCalledTimes(1);
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(onReconnect).toHaveBeenCalledTimes(1);
  });

  it("case 7: repeated errors before one open refetch exactly once", async () => {
    const onPin = vi.fn();
    const onReconnect = vi.fn();
    await act(async () => {
      root.render(createElement(StreamProbe, { bbox: "1,2,3,4", onPin, onReconnect }));
    });
    const es = FakeEventSource.instances[0];
    await act(async () => {
      es.fail();
      es.fail();
      es.fail();
    });
    expect(onReconnect).not.toHaveBeenCalled();
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(onReconnect).not.toHaveBeenCalled();
    await act(async () => {
      es.open();
    });
    expect(onReconnect).toHaveBeenCalledTimes(1);
  });
});
