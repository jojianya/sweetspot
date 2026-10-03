// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePinAddress } from "./usePinAddress";

const geoMocks = vi.hoisted(() => ({
  reverseGeocode: vi.fn(),
}));

vi.mock("@/lib/api/geocoding", () => geoMocks);

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function AddressProbe({ point }: { point: { lat: number; lng: number } | null }) {
  const address = usePinAddress(point);
  return <span>{address ?? "none"}</span>;
}

describe("usePinAddress", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    geoMocks.reverseGeocode.mockReset();
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

  it("resolves to the geocoded address", async () => {
    geoMocks.reverseGeocode.mockResolvedValue("MG Road, Bengaluru");
    await act(async () => {
      root.render(createElement(AddressProbe, { point: { lat: 12.9, lng: 77.6 } }));
    });
    expect(geoMocks.reverseGeocode).toHaveBeenCalledTimes(1);
    expect(container.textContent).toBe("MG Road, Bengaluru");
  });

  it("swallows lookup failures and stays null", async () => {
    geoMocks.reverseGeocode.mockRejectedValue(new Error("network unavailable"));
    await act(async () => {
      root.render(createElement(AddressProbe, { point: { lat: 12.9, lng: 77.6 } }));
    });
    expect(container.textContent).toBe("none");
  });

  it("does nothing without a point", async () => {
    await act(async () => {
      root.render(createElement(AddressProbe, { point: null }));
    });
    expect(geoMocks.reverseGeocode).not.toHaveBeenCalled();
    expect(container.textContent).toBe("none");
  });

  it("ignores a superseded lookup that resolves late", async () => {
    const first = deferred<string | null>();
    geoMocks.reverseGeocode.mockImplementationOnce(() => first.promise);
    geoMocks.reverseGeocode.mockResolvedValueOnce("Second Street");
    await act(async () => {
      root.render(createElement(AddressProbe, { point: { lat: 1, lng: 1 } }));
    });
    await act(async () => {
      root.render(createElement(AddressProbe, { point: { lat: 2, lng: 2 } }));
    });
    await act(async () => {
      first.resolve("First Street (stale)");
      await first.promise;
    });
    expect(container.textContent).toBe("Second Street");
  });
});
