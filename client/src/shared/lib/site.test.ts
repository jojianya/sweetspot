import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resolveSiteUrl } from "./site";

// resolveSiteUrl reads process.env through default parameters:
//
//   resolveSiteUrl(siteUrl = process.env.SITE_URL, apiUrl = process.env.NEXT_PUBLIC_API_URL)
//
// Passing `undefined` explicitly does NOT mean "not configured" — it triggers
// the default and pulls the value from the environment. So these tests used to
// pass or fail depending on whether SITE_URL happened to be set, which is why
// they were green locally and red in CI (ci.yml sets SITE_URL for the test
// step). Pinning the env here keeps every case below deterministic and makes
// the intent of each call explicit.
describe("resolveSiteUrl", () => {
  beforeEach(() => {
    vi.stubEnv("SITE_URL", "");
    vi.stubEnv("NEXT_PUBLIC_API_URL", "");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

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

  it("falls back to the env when no origin is passed at all", () => {
    // The real production path: layout.tsx calls getSiteUrl() with no args.
    vi.stubEnv("SITE_URL", "https://goodspot.example");
    expect(resolveSiteUrl().origin).toBe("https://goodspot.example");
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

  it("treats an empty site origin as not configured", () => {
    // An empty string is falsy after trim, so it reaches the API-origin check
    // rather than being accepted as a valid origin.
    expect(() => resolveSiteUrl("", "https://api.example.com")).toThrow(
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
