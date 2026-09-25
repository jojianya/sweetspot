import { beforeEach, describe, expect, it, vi } from "vitest";
import { fetchUser } from "./users";

const apiMocks = vi.hoisted(() => ({ get: vi.fn() }));

vi.mock("./client", () => ({ default: apiMocks }));

const profile = {
  id: "user-1",
  username: "alice",
  avatar_url: null,
  socials: {},
  role: "user",
  created_at: "2026-01-01T00:00:00Z",
};

describe("fetchUser", () => {
  beforeEach(() => {
    apiMocks.get.mockReset();
  });

  it("validates the public profile payload", async () => {
    apiMocks.get.mockResolvedValue({ data: profile });

    await expect(fetchUser("user-1")).resolves.toEqual(profile);
    expect(apiMocks.get).toHaveBeenCalledWith("/users/user-1");
  });

  it("rejects a malformed public profile payload", async () => {
    apiMocks.get.mockResolvedValue({ data: { ...profile, username: null } });

    await expect(fetchUser("user-1")).rejects.toThrow();
  });
});
