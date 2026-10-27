import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

describe("geocoding key guard", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.stubEnv("NEXT_PUBLIC_MAPTILER_API_KEY", "");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it("returns [] without fetching when the key is missing", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const mod = await import("./geocoding");
    expect(mod.isGeocodingAvailable()).toBe(false);
    await expect(mod.searchPlaces("market")).resolves.toEqual([]);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("returns null without fetching for reverse geocode when the key is missing", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const mod = await import("./geocoding");
    await expect(mod.reverseGeocode({ lat: 0, lng: 0 })).resolves.toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("reports available when a key is configured", async () => {
    vi.stubEnv("NEXT_PUBLIC_MAPTILER_API_KEY", "test-key");
    const mod = await import("./geocoding");
    expect(mod.isGeocodingAvailable()).toBe(true);
  });
});
