import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AxiosAdapter } from "axios";
import api, { ApiError } from "./client";
import { errorMessage } from "@/lib/utils";
import { useAuth } from "@/store/auth";

describe("ApiError", () => {
  it("carries status, code, and message", () => {
    const err = new ApiError("not found", 404, "NotFound");
    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("not found");
    expect(err.status).toBe(404);
    expect(err.code).toBe("NotFound");
    expect(err.name).toBe("ApiError");
  });

  it("defaults code to undefined when not provided", () => {
    const err = new ApiError("boom", 500);
    expect(err.status).toBe(500);
    expect(err.code).toBeUndefined();
  });

  it("uses status 0 when there is no HTTP response", () => {
    // Network errors, timeouts, and CORS failures never reach the server,
    // so there is no status to read. The interceptor defaults to 0.
    const err = new ApiError("network error", 0);
    expect(err.status).toBe(0);
  });

  it("is distinguishable by status, not by message text", () => {
    // The whole point: a 404 and a 400 with the same message are different.
    const notFound = new ApiError("not found", 404);
    const badRequest = new ApiError("not found", 400);
    const serverError = new ApiError("not found", 500);

    expect(notFound.status).toBe(404);
    expect(badRequest.status).toBe(400);
    expect(serverError.status).toBe(500);

    // All three have the same message but different statuses.
    expect(notFound.message).toBe(badRequest.message);
    expect(notFound.status).not.toBe(badRequest.status);
  });

  it("works with errorMessage() for display", () => {
    const err = new ApiError("user not found", 404);
    expect(errorMessage(err)).toBe("user not found");
  });

  it("is an instance of Error so existing catch blocks still work", () => {
    const err = new ApiError("boom", 500);
    expect(err instanceof Error).toBe(true);
    expect(err instanceof ApiError).toBe(true);
  });
});

describe("401 session backstop", () => {
  // Rejects like an axios adapter failure with the given status and URL.
  // Null status simulates a network error (no response at all).
  function failWith(status: number | null, url?: string): AxiosAdapter {
    return () =>
      Promise.reject({
        response: status === null ? undefined : { status, data: { error: "denied" } },
        config: { url },
        message:
          status === null ? "Network Error" : `Request failed with status code ${status}`,
      });
  }

  function signedIn() {
    useAuth.setState({
      user: { id: "user-1", username: "alice", avatar_url: null, role: "user" },
    });
  }

  beforeEach(() => {
    signedIn();
  });

  afterEach(() => {
    useAuth.setState({ user: null });
    vi.restoreAllMocks();
  });

  it("clears the session on a 401 from an authenticated request", async () => {
    const failed = api
      .get("/feed", { adapter: failWith(401, "/feed") })
      .catch((e) => e);
    await expect(failed).resolves.toMatchObject({ status: 401 });
    expect(useAuth.getState().user).toBeNull();
  });

  it("keeps the session on a 401 from login or register", async () => {
    for (const url of ["/auth/login", "/auth/register"]) {
      signedIn();
      await expect(
        api.get(url, { adapter: failWith(401, url) }).catch((e) => e)
      ).resolves.toMatchObject({ status: 401 });
      expect(useAuth.getState().user).not.toBeNull();
    }
  });

  it("keeps clearing on a 401 from password endpoints, as today", async () => {
    await expect(
      api.get("/auth/password/request", { adapter: failWith(401, "/auth/password/request") }).catch((e) => e)
    ).resolves.toMatchObject({ status: 401 });
    expect(useAuth.getState().user).toBeNull();
  });

  it("leaves the session alone on non-401 errors", async () => {
    await expect(
      api.get("/feed", { adapter: failWith(403, "/feed") }).catch((e) => e)
    ).resolves.toMatchObject({ status: 403 });
    expect(useAuth.getState().user).not.toBeNull();
  });

  it("leaves the session alone on network errors", async () => {
    await expect(
      api.get("/feed", { adapter: failWith(null, "/feed") }).catch((e) => e)
    ).resolves.toMatchObject({ status: 0 });
    expect(useAuth.getState().user).not.toBeNull();
  });

  it("settles concurrent 401s the same way, without throwing", async () => {
    const results = await Promise.allSettled([
      api.get("/feed", { adapter: failWith(401, "/feed") }),
      api.get("/feed", { adapter: failWith(401, "/feed") }),
    ]);
    expect(results.map((r) => r.status)).toEqual(["rejected", "rejected"]);
    expect(useAuth.getState().user).toBeNull();
  });

  it("keeps the thrown error shape callers rely on", async () => {
    const err = await api
      .get("/feed", { adapter: failWith(401, "/feed") })
      .catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("denied");
    expect(err.status).toBe(401);
  });
});
