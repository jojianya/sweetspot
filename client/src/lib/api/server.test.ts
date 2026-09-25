import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PinDetail } from "@/lib/types";
import { fetchPinServer } from "./server";

const PIN_ID = "11111111-1111-4111-8111-111111111111";

const pin: PinDetail = {
  id: PIN_ID,
  user_id: "22222222-2222-4222-8222-222222222222",
  location: "POINT(0 0)",
  geohash: "s000",
  caption: "A great spot",
  category_id: 1,
  is_hidden: false,
  views: 4,
  created_at: "2026-01-02T00:00:00Z",
  category: "Food",
  username: "alice",
  avatar_url: null,
  photos: [],
};

const fetchMock = vi.fn();

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("fetchPinServer", () => {
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("loads and validates a public pin", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ pin }));

    await expect(fetchPinServer(PIN_ID)).resolves.toEqual(pin);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(new RegExp(`/pins/${PIN_ID}$`)),
      expect.objectContaining({ cache: "no-store" })
    );
  });

  it("returns null only for a genuine not-found response", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: "pin not found" }, 404));

    await expect(fetchPinServer(PIN_ID)).resolves.toBeNull();
  });

  it("does not turn upstream failures into not-found responses", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: "boom" }, 500));
    await expect(fetchPinServer(PIN_ID)).rejects.toThrow("HTTP 500");

    fetchMock.mockRejectedValueOnce(new DOMException("timed out", "TimeoutError"));
    await expect(fetchPinServer(PIN_ID)).rejects.toThrow("timed out");
  });

  it("rejects malformed successful responses", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ pin: { id: PIN_ID } }));
    await expect(fetchPinServer(PIN_ID)).rejects.toThrow();

    fetchMock.mockResolvedValueOnce(new Response("not json", { status: 200 }));
    await expect(fetchPinServer(PIN_ID)).rejects.toThrow();
  });
});
