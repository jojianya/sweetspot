import { describe, expect, it } from "vitest";
import { parsePoint } from "./geo";

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