import { describe, expect, it } from "vitest";
import { isSessionEnded } from "./session";
import { privateUserSchema, publicUserSchema } from "./schemas";

describe("isSessionEnded", () => {
  // These are the endpoints behind the bug report: opening any of them with a
  // stale cached user produced a 401 that read as "clicking this page logged
  // me out".
  const sessionProtected = [
    "/feed",
    "/favorites",
    "/favorites/ids",
    "/users",
    "/reports",
    "/me",
    "/users/ebfdafc9-94e5-4005-a27d-5126d315d3f4",
  ];

  it.each(sessionProtected)(
    "treats a 401 from %s as an ended session",
    (url) => {
      expect(isSessionEnded(url)).toBe(true);
    }
  );

  // A failed login is a 401, but the caller has no session to lose. Clearing
  // here would sign the user out for mistyping a password.
  it("does not treat a failed login as an ended session", () => {
    expect(isSessionEnded("/auth/login")).toBe(false);
  });

  it("does not treat a failed registration as an ended session", () => {
    expect(isSessionEnded("/auth/register")).toBe(false);
  });

  it("does not match a credential path that only appears in the prefix", () => {
    // Guards against a future absolute baseURL making this match by accident.
    expect(isSessionEnded("/pins/abc/report")).toBe(true);
  });

  it("does not clear auth when the URL is unknown", () => {
    // Guessing wrong here would silently sign the user out, so an absent URL
    // is treated as "not a session failure".
    expect(isSessionEnded(undefined)).toBe(false);
    expect(isSessionEnded("")).toBe(false);
  });
});

describe("profile schemas document why GET /users/:id cannot validate a session", () => {
  // GET /users/:id sits behind OptionalAuth: it answers 200 even with no
  // cookie, and returns the *public* shape rather than the caller's own. The
  // refresh hook parsed that with privateUserSchema, so a dead session threw a
  // ZodError that its empty catch swallowed, leaving the stale user cached.
  const publicShape = {
    id: "ebfdafc9-94e5-4005-a27d-5126d315d3f4",
    username: "onwertest",
    avatar_url: null,
    socials: {},
    role: "owner",
    created_at: "2026-09-27T16:11:16Z",
  };

  it("accepts the unauthenticated response as a public profile", () => {
    expect(publicUserSchema.parse(publicShape)).toEqual(publicShape);
  });

  it("rejects that same response as a private profile", () => {
    // This is the swallowed throw. Pinning it so nobody reintroduces an
    // OptionalAuth endpoint as a session check.
    expect(() => privateUserSchema.parse(publicShape)).toThrow();
  });
});
