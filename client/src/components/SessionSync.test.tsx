// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AxiosAdapter } from "axios";
import api, { setUnauthorizedHandler } from "@/lib/api/client";
import { useAuth } from "@/store/auth";
import SessionSync from "./SessionSync";

vi.mock("@/hooks/useSessionSync", () => ({
  useSessionSync: () => {},
}));

function failWith(status: number, url: string): AxiosAdapter {
  return () =>
    Promise.reject({
      response: { status, data: { error: "denied" } },
      config: { url },
      message: `Request failed with status code ${status}`,
    });
}

describe("SessionSync registration", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    useAuth.setState({
      user: { id: "user-1", username: "alice", avatar_url: null, role: "user" },
    });
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
    setUnauthorizedHandler(() => {});
  });

  it("clears the session on a 401 while mounted", async () => {
    await act(async () => {
      root.render(createElement(SessionSync));
    });
    await expect(
      api.get("/feed", { adapter: failWith(401, "/feed") }).catch((e) => e)
    ).resolves.toMatchObject({ status: 401 });
    expect(useAuth.getState().user).toBeNull();
  });

  it("stops clearing after unmount (cleanup restores the no-op)", async () => {
    await act(async () => {
      root.render(createElement(SessionSync));
    });
    await act(async () => {
      root.unmount();
    });
    useAuth.setState({
      user: { id: "user-1", username: "alice", avatar_url: null, role: "user" },
    });
    await expect(
      api.get("/feed", { adapter: failWith(401, "/feed") }).catch((e) => e)
    ).resolves.toMatchObject({ status: 401 });
    expect(useAuth.getState().user).not.toBeNull();
  });
});
