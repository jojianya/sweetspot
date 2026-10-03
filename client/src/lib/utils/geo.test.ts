import { describe, expect, it } from "vitest";
import type { LngLatBounds } from "maplibre-gl";
import {
  boundsToValidBbox,
  neighborhoodBounds,
  parsePoint,
  PIN_NEIGHBOR_LIMIT,
  selectNearbyPins,
} from "./geo";

describe("parsePoint", () => {
  it("parses a valid WKT point", () => {
    expect(parsePoint("POINT(78.4867 17.385)")).toEqual({
      lng: 78.4867,
      lat: 17.385,
    });
  });

  it("accepts negative coordinates", () => {
    expect(parsePoint("POINT (-122.4 -37.8)")).toEqual({ lng: -122.4, lat: -37.8 });
  });

  it("returns null for a malformed WKT string", () => {
    expect(parsePoint("not a point")).toBeNull();
    expect(parsePoint("")).toBeNull();
  });

  it("returns null for out-of-range coordinates", () => {
    expect(parsePoint("POINT(181 17)")).toBeNull();
    expect(parsePoint("POINT(78 91)")).toBeNull();
  });

  it("returns null for non-numeric coordinates", () => {
    expect(parsePoint("POINT(abc def)")).toBeNull();
  });
});

describe("selectNearbyPins", () => {
  const pin = (id: string, lng: number, lat: number) => ({
    id,
    location: `POINT(${lng} ${lat})`,
  });

  it("returns the clicked pin followed by the nearest pins within the radius", () => {
    const pins = [
      pin("clicked", 0, 0),
      pin("near-1", 0.009, 0),
      pin("near-2", 0.001, 0),
      pin("far", 0.1, 0),
    ];

    const result = selectNearbyPins("clicked", pins);

    expect(result.map(({ id }) => id)).toEqual(["clicked", "near-2", "near-1"]);
  });

  it("caps the neighborhood at the configured neighbor limit", () => {
    const pins = [
      pin("clicked", 0, 0),
      ...Array.from({ length: PIN_NEIGHBOR_LIMIT + 3 }, (_, index) =>
        pin(`near-${index}`, (index + 1) * 0.001, 0)
      ),
    ];

    const result = selectNearbyPins("clicked", pins);

    expect(result).toHaveLength(PIN_NEIGHBOR_LIMIT + 1);
    expect(result[0].id).toBe("clicked");
  });

  it("returns only the clicked pin when no neighbor is in range", () => {
    const result = selectNearbyPins("clicked", [
      pin("clicked", 0, 0),
      pin("far", 0.1, 0),
    ]);

    expect(result.map(({ id }) => id)).toEqual(["clicked"]);
  });

  it("returns an empty result when the clicked pin is not loaded", () => {
    expect(selectNearbyPins("missing", [pin("loaded", 0, 0)])).toEqual([]);
  });
});

describe("boundsToValidBbox", () => {
  const bounds = (south: number, west: number, north: number, east: number) =>
    ({
      getSouth: () => south,
      getWest: () => west,
      getNorth: () => north,
      getEast: () => east,
    }) as unknown as LngLatBounds;

  it("quantizes coordinates to ~11m so identical views emit identical strings", () => {
    const [south, west, north, east] = boundsToValidBbox(
      bounds(12.345678, 77.654321, 12.365678, 77.674321)
    );
    // Math.round(x * 10000) / 10000 carries binary float artifacts
    // (77.65430000000003); the server parses floats so this is harmless,
    // but the values must be stable and within half a quantum.
    for (const [actual, want] of [
      [south, 12.3457],
      [west, 77.6543],
      [north, 12.3657],
      [east, 77.6743],
    ] as const) {
      expect(actual).toBeCloseTo(want, 4);
    }
    expect(boundsToValidBbox(bounds(12.345678, 77.654321, 12.365678, 77.674321))).toEqual([
      south, west, north, east,
    ]);
  });

  it("clamps latitudes to the valid range", () => {
    expect(boundsToValidBbox(bounds(-95, 0, 95, 10))).toEqual([-90, 0, 90, 10]);
  });

  it("expands a full-world span to the whole longitude range", () => {
    expect(boundsToValidBbox(bounds(-10, -200, 10, 200))).toEqual([-10, -180, 10, 180]);
  });
});

describe("neighborhoodBounds", () => {
  it("returns null for fewer than two points", () => {
    expect(neighborhoodBounds([])).toBeNull();
    expect(neighborhoodBounds([{ lat: 1, lng: 2 }])).toBeNull();
  });

  it("frames spread points exactly", () => {
    expect(
      neighborhoodBounds([
        { lat: 10, lng: 20 },
        { lat: 12, lng: 26 },
      ])
    ).toEqual({ west: 20, south: 10, east: 26, north: 12 });
  });

  it("expands coincident points to the minimum span around their center", () => {
    expect(
      neighborhoodBounds([
        { lat: 10, lng: 20 },
        { lat: 10, lng: 20 },
      ])
    ).toEqual({ west: 19.995, south: 9.995, east: 20.005, north: 10.005 });
  });

  it("expands only the tight axis", () => {
    expect(
      neighborhoodBounds([
        { lat: 10, lng: 20 },
        { lat: 10, lng: 26 },
      ])
    ).toEqual({ west: 20, south: 9.995, east: 26, north: 10.005 });
  });
});
