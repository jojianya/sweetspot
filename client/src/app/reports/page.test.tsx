// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { reportEmptyCopy } from "./page";
import { useAuth, type PersistedUser } from "@/store/auth";

const apiMocks = vi.hoisted(() => ({
  fetchSession: vi.fn(),
  fetchReports: vi.fn(),
  reviewReport: vi.fn(),
}));

vi.mock("@/lib/api/auth", () => ({ fetchSession: apiMocks.fetchSession }));
vi.mock("@/lib/api", () => ({
  fetchReports: apiMocks.fetchReports,
  reviewReport: apiMocks.reviewReport,
}));
// Navbar pulls in useRouter through useLogout.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("@/lib/api/users", () => ({
  fetchMe: vi.fn().mockRejectedValue(new Error("offline")),
}));

import ReportsPage from "./page";

describe("reportEmptyCopy", () => {
  it("keeps the per-status wording", () => {
    expect(reportEmptyCopy("pending")).toEqual({
      title: "No pending reports",
      description: "You're all caught up.",
    });
    expect(reportEmptyCopy("reviewed")).toEqual({
      title: "No reviewed reports",
      description: "Try another filter.",
    });
    expect(reportEmptyCopy("actioned")).toEqual({
      title: "No actioned reports",
      description: "Try another filter.",
    });
  });

  it("does not tell All-tab viewers to try another filter", () => {
    expect(reportEmptyCopy("all")).toEqual({
      title: "No reports",
      description: "No reports have been filed yet.",
    });
  });
});

describe("ReportsPage authorization", () => {
  let container: HTMLDivElement;
  let root: Root;

  // Seed localStorage directly: this is the tampered-cache case, where the
  // persisted role claims owner but the server disagrees.
  function seedCachedUser(user: PersistedUser | null) {
    window.localStorage.setItem(
      "goodspot-auth",
      JSON.stringify({ state: { user }, version: 0 })
    );
  }

  async function render() {
    await act(async () => {
      root.render(<ReportsPage />);
    });
  }

  beforeEach(async () => {
    apiMocks.fetchSession.mockReset().mockResolvedValue({ user_id: "u-1", role: "owner" });
    apiMocks.fetchReports.mockReset().mockResolvedValue([]);
    apiMocks.reviewReport.mockReset().mockResolvedValue(undefined);
    window.localStorage.clear();
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    // Start from a clean store so each case reads only what it seeded.
    useAuth.setState({ user: null });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    window.localStorage.clear();
  });

  it("does not render the moderation queue or fetch reports when /me says the cached owner role is stale", async () => {
    seedCachedUser({ id: "u-1", username: "mallory", avatar_url: null, role: "owner" });
    await act(async () => {
      useAuth.setState({
        user: { id: "u-1", username: "mallory", avatar_url: null, role: "owner" },
      });
    });
    // The server says this is an ordinary user.
    apiMocks.fetchSession.mockResolvedValue({ user_id: "u-1", role: "user" });

    await render();

    expect(container.textContent).toContain("Admins only");
    // The moderation queue's own heading and the role link into it stay out of
    // the tree for a caller the server did not authorize.
    expect(container.querySelector("h1")?.textContent).toBe("Admins only");
    expect(apiMocks.fetchReports).not.toHaveBeenCalled();
  });

  it("renders the queue and fetches reports when /me confirms the role", async () => {
    await act(async () => {
      useAuth.setState({
        user: { id: "u-1", username: "alice", avatar_url: null, role: "owner" },
      });
    });

    await render();

    expect(container.textContent).toContain("Reports");
    expect(apiMocks.fetchReports).toHaveBeenCalledWith({ status: "pending" });
  });

  it("does not fetch reports for a signed-out caller", async () => {
    apiMocks.fetchSession.mockRejectedValue(new Error("no session"));
    await render();

    expect(container.textContent).toContain("Sign in to moderate");
    expect(apiMocks.fetchReports).not.toHaveBeenCalled();
  });
});