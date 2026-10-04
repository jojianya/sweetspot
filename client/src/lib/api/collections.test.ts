import { beforeEach, describe, expect, it, vi } from "vitest";
import { createCollection, updateCollection } from "./collections";

const apiMocks = vi.hoisted(() => ({ post: vi.fn(), patch: vi.fn() }));

vi.mock("./client", () => ({ default: apiMocks }));

const entry = {
  id: "col-1",
  user_id: "user-1",
  name: "Trip",
  description: null,
  is_private: false,
  created_at: "2026-01-02T00:00:00Z",
  pin_count: 0,
  cover_url: "",
};

describe("collection privacy payloads", () => {
  beforeEach(() => {
    apiMocks.post.mockReset();
    apiMocks.patch.mockReset();
  });

  it("creates public by default", async () => {
    apiMocks.post.mockResolvedValue({ data: { collection: entry } });
    await expect(createCollection("Trip")).resolves.toEqual(entry);
    expect(apiMocks.post).toHaveBeenCalledWith("/collections", {
      name: "Trip",
      description: null,
      is_private: false,
    });
  });

  it("creates private when asked", async () => {
    apiMocks.post.mockResolvedValue({ data: { collection: { ...entry, is_private: true } } });
    await expect(createCollection("Trip", null, true)).resolves.toMatchObject({ is_private: true });
    expect(apiMocks.post).toHaveBeenCalledWith("/collections", {
      name: "Trip",
      description: null,
      is_private: true,
    });
  });

  it("sends is_private only when set on update", async () => {
    apiMocks.patch.mockResolvedValue({ data: {} });
    await updateCollection("col-1", { name: "Trip" });
    expect(apiMocks.patch).toHaveBeenCalledWith("/collections/col-1", {
      name: "Trip",
      description: null,
    });
    await updateCollection("col-1", { name: "Trip", isPrivate: true });
    expect(apiMocks.patch).toHaveBeenCalledWith("/collections/col-1", {
      name: "Trip",
      description: null,
      is_private: true,
    });
  });

  it("rejects payloads missing is_private", async () => {
    apiMocks.post.mockResolvedValue({ data: { collection: { ...entry, is_private: undefined } } });
    await expect(createCollection("Trip")).rejects.toThrow();
  });
});
