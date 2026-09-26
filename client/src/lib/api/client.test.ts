import { describe, expect, it } from "vitest";
import { ApiError } from "./client";
import { errorMessage } from "@/lib/utils";

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
