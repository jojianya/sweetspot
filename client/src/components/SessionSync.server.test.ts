import { describe, expect, it } from "vitest";
import type { AxiosAdapter } from "axios";
import api from "@/lib/api/client";
import { useAuth } from "@/store/auth";
// Importing the component module (not rendering it) must be safe without a
// browser: registration happens in an effect, so effects never run here.
import "@/components/SessionSync";

function failWith(status: number, url: string): AxiosAdapter {
  return () =>
    Promise.reject({
      response: { status, data: { error: "denied" } },
      config: { url },
      message: `Request failed with status code ${status}`,
    });
}

describe("server-side import safety", () => {
  it("does not touch the store on import and leaves 401s uncleared", async () => {
    expect(typeof window).toBe("undefined");
    useAuth.setState({
      user: { id: "user-1", username: "alice", avatar_url: null, role: "user" },
    });
    await expect(
      api.get("/feed", { adapter: failWith(401, "/feed") }).catch((e) => e)
    ).resolves.toMatchObject({ status: 401 });
    // No effect ran, so the default no-op handler held and the user stayed.
    expect(useAuth.getState().user).not.toBeNull();
    useAuth.setState({ user: null });
  });
});
