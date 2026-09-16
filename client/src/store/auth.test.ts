// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { useAuth } from "./auth";

const fullUser = {
  id: "u1",
  email: "secret@example.com",
  username: "alice",
  avatar_url: null,
  socials: {},
  role: "user",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

function readPersisted() {
  const raw = localStorage.getItem("goodspot-auth");
  expect(raw).not.toBeNull();
  return JSON.parse(raw ?? "");
}

describe("auth store", () => {
  it("persists only the token and a pruned user without email", async () => {
    useAuth.getState().clearAuth();
    useAuth.getState().setAuth(fullUser, "tok.123");

    await vi.waitFor(() => {
      const stored = readPersisted();
      expect(stored.state.token).toBe("tok.123");
      expect(stored.state.user.email).toBeUndefined();
      expect(stored.state.user.username).toBe("alice");
      expect(stored.state.user.id).toBe("u1");
    });
  });

  it("clears the persisted token on logout", async () => {
    useAuth.getState().setAuth(fullUser, "tok.123");
    useAuth.getState().clearAuth();

    await vi.waitFor(() => {
      const stored = readPersisted();
      expect(stored.state.token).toBeNull();
      expect(stored.state.user).toBeNull();
    });
  });

  it("keeps the action methods out of persisted state", async () => {
    useAuth.getState().setAuth(fullUser, "tok.123");

    await vi.waitFor(() => {
      const stored = readPersisted();
      expect(stored.state.setAuth).toBeUndefined();
      expect(stored.state.clearAuth).toBeUndefined();
    });
  });
});