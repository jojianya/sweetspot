import { describe, expect, it } from "vitest";

import { DEFAULT_DEV_ORIGINS, normalizeDevOrigin, resolveAllowedDevOrigins } from "./devOrigins";

describe("normalizeDevOrigin", () => {
  it("keeps a bare hostname", () => {
    expect(normalizeDevOrigin("192.168.68.112")).toBe("192.168.68.112");
  });

  it("trims surrounding whitespace", () => {
    expect(normalizeDevOrigin("  box.local  ")).toBe("box.local");
  });

  it("lowercases the hostname", () => {
    expect(normalizeDevOrigin("Box.Local")).toBe("box.local");
  });

  it("strips a scheme", () => {
    expect(normalizeDevOrigin("http://192.168.68.112")).toBe("192.168.68.112");
    expect(normalizeDevOrigin("https://box.local")).toBe("box.local");
  });

  it("strips a port", () => {
    expect(normalizeDevOrigin("http://192.168.68.112:3000")).toBe("192.168.68.112");
    expect(normalizeDevOrigin("192.168.68.112:3000")).toBe("192.168.68.112");
  });

  it("strips a trailing slash and path", () => {
    expect(normalizeDevOrigin("http://box.local:3000/")).toBe("box.local");
    expect(normalizeDevOrigin("box.local/")).toBe("box.local");
  });

  it("keeps IPv6 addresses usable", () => {
    // url.hostname keeps IPv6 in bracketed form, which is what Next expects.
    expect(normalizeDevOrigin("http://[fd00::1]:3000")).toBe("[fd00::1]");
  });

  it("preserves the documented wildcard form", () => {
    expect(normalizeDevOrigin("*.example.dev")).toBe("*.example.dev");
    expect(normalizeDevOrigin("*.EXAMPLE.dev")).toBe("*.example.dev");
  });

  it("returns null for blank entries", () => {
    expect(normalizeDevOrigin("")).toBeNull();
    expect(normalizeDevOrigin("   ")).toBeNull();
    expect(normalizeDevOrigin("/")).toBeNull();
  });
});

describe("resolveAllowedDevOrigins", () => {
  it("falls back to the defaults when unset", () => {
    expect(resolveAllowedDevOrigins(undefined)).toEqual([...DEFAULT_DEV_ORIGINS]);
  });

  it("falls back to the defaults when empty", () => {
    expect(resolveAllowedDevOrigins("")).toEqual([...DEFAULT_DEV_ORIGINS]);
    expect(resolveAllowedDevOrigins("   ")).toEqual([...DEFAULT_DEV_ORIGINS]);
  });

  it("falls back to the defaults when only separators are present", () => {
    expect(resolveAllowedDevOrigins(",,")).toEqual([...DEFAULT_DEV_ORIGINS]);
    expect(resolveAllowedDevOrigins(" , , ")).toEqual([...DEFAULT_DEV_ORIGINS]);
  });

  it("splits a comma-separated list", () => {
    expect(resolveAllowedDevOrigins("localhost,127.0.0.1,192.168.68.112")).toEqual([
      "localhost",
      "127.0.0.1",
      "192.168.68.112",
    ]);
  });

  it("normalizes each entry and drops blanks", () => {
    expect(resolveAllowedDevOrigins(" http://box.local:3000/ , ,192.168.68.112 ")).toEqual([
      "box.local",
      "192.168.68.112",
    ]);
  });

  it("replaces the defaults rather than appending to them", () => {
    // Listing a LAN address must not require re-listing localhost, and must not
    // silently keep the defaults either — the value is the whole list.
    expect(resolveAllowedDevOrigins("192.168.68.112")).toEqual(["192.168.68.112"]);
  });

  it("deduplicates after normalization", () => {
    // Different spellings of the same host collapse, so listing it twice does
    // not bloat the list Next has to match against.
    expect(resolveAllowedDevOrigins("box.local,BOX.local,http://box.local:3000")).toEqual([
      "box.local",
    ]);
  });
});