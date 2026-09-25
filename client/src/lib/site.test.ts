import { describe, expect, it } from "vitest";
import { resolveSiteUrl } from "./site";

describe("resolveSiteUrl", () => {
  it("uses an explicit public HTTPS origin", () => {
    expect(resolveSiteUrl("https://goodspot.example/").origin).toBe(
      "https://goodspot.example"
    );
  });

  it("allows HTTP loopback origins for local development", () => {
    expect(resolveSiteUrl("http://localhost:3000").origin).toBe(
      "http://localhost:3000"
    );
  });

  it("defaults to localhost only when no public API origin is configured", () => {
    expect(resolveSiteUrl(undefined, undefined).origin).toBe(
      "http://localhost:3000"
    );
    expect(resolveSiteUrl(undefined, "http://localhost:8081").origin).toBe(
      "http://localhost:3000"
    );
  });

  it("rejects a public API origin without an explicit site origin", () => {
    expect(() => resolveSiteUrl(undefined, "https://api.example.com")).toThrow(
      "SITE_URL is required"
    );
  });

  it("rejects insecure or non-origin public values", () => {
    expect(() => resolveSiteUrl("http://goodspot.example")).toThrow("must use https");
    expect(() => resolveSiteUrl("https://goodspot.example/app")).toThrow(
      "must not include a path"
    );
    expect(() => resolveSiteUrl("https://goodspot.example?preview=1")).toThrow(
      "must be an origin"
    );
  });
});
