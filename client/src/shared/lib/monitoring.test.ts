// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { _resetForTests, API_BASE_URL, reportError } from "./monitoring";

function stubFetch() {
  const fetchMock = vi
    .fn()
    .mockResolvedValue(new Response(null, { status: 204 }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

/** Points jsdom's location at a URL with a query string and a fragment. */
function setLocation(href: string) {
  window.history.replaceState({}, "", href);
}

describe("reportError", () => {
  beforeEach(() => {
    _resetForTests();
    setLocation("/");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts an error report to the server ingest endpoint", () => {
    const fetchMock = stubFetch();

    reportError(new Error("boom"), { kind: "api", url: "/pins" });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${API_BASE_URL}/errors`);
    expect(init.method).toBe("POST");
    expect(init.headers["Content-Type"]).toBe("application/json");

    const body = JSON.parse(init.body as string);
    expect(body.message).toBe("boom");
    expect(body.stack).toBeDefined();
    expect(body.extra).toEqual({ kind: "api", url: "/pins" });
  });

  it("dedupes identical failures within the throttle window", () => {
    const fetchMock = stubFetch();

    reportError(new Error("boom"));
    reportError(new Error("boom")); // same message, no url -> deduped
    reportError(new Error("bang"));

    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("keeps distinct recipients visible within the throttle window", () => {
    const fetchMock = stubFetch();

    reportError(new Error("boom"), { kind: "api", url: "/pins/a" });
    reportError(new Error("boom"), { kind: "api", url: "/pins/b" });

    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("caps the number of events reported per session", () => {
    const fetchMock = stubFetch();

    for (let i = 0; i < 150; i++) {
      reportError(new Error(`err-${i}`));
    }

    expect(fetchMock).toHaveBeenCalledTimes(100);
  });

  it("coerces non-Error values to a message and never throws", () => {
    const fetchMock = stubFetch();

    expect(() => reportError(42)).not.toThrow();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const init = fetchMock.mock.calls[0][1];
    const body = JSON.parse(init.body as string);
    expect(body.message).toBe("42");
  });

  // The reported URL reaches the server's logs and Sentry, so a token or
  // invite code in the query string or fragment must never be sent.
  it("reports the origin and path without the query string or fragment", () => {
    const fetchMock = stubFetch();
    setLocation("/pins/abc123?token=secret-token#section");

    reportError(new Error("boom"));

    const init = fetchMock.mock.calls[0][1];
    const body = JSON.parse(init.body as string);
    expect(body.url).toBe(`${window.location.origin}/pins/abc123`);
    expect(body.url).not.toContain("secret-token");
    expect(body.url).not.toContain("?");
    expect(body.url).not.toContain("#");
  });

  it("omits the url when there is no window", () => {
    const fetchMock = stubFetch();
    vi.stubGlobal("window", undefined);

    reportError(new Error("boom"));

    const init = fetchMock.mock.calls[0][1];
    const body = JSON.parse(init.body as string);
    expect(body.url).toBeUndefined();
  });
});