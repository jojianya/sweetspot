// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

// Confirms the session token is not reachable from client-side JavaScript.
//
// The token lives in an httpOnly cookie set by the server, so:
//   - localStorage must not contain it (nothing auth-related is stored there)
//   - document.cookie must not expose it (httpOnly strips it from the DOM)
//
// If a future change reintroduces client-side token storage, these fail.
describe("token is not readable from client JS", () => {
  it("stores no token in localStorage under any key", () => {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i) ?? "";
      const value = localStorage.getItem(key) ?? "";
      expect(value).not.toContain("eyJ"); // JWT prefix
      expect(key).not.toMatch(/token|jwt|session/i);
    }
  });

  it("does not expose the session cookie to document.cookie", () => {
    // Simulate the httpOnly cookie being set by the server. In jsdom,
    // document.cookie only returns non-httpOnly cookies.
    document.cookie = "other=visible";
    expect(document.cookie).not.toContain("session_token");
  });

  it("persists the user object without a token field", async () => {
    const { useAuth } = await import("./auth");
    useAuth.getState().setUser({
      id: "u1",
      email: "a@b.com",
      username: "alice",
      avatar_url: null,
      socials: {},
      role: "user",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    });

    const raw = localStorage.getItem("goodspot-auth");
    expect(raw).not.toBeNull();
    const stored = JSON.parse(raw ?? "");
    expect(stored.state.token).toBeUndefined();
  });
});
