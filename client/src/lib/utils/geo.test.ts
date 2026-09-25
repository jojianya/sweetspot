import { describe, expect, it } from "vitest";
import {
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
