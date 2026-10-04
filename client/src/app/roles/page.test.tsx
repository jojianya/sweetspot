// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";

const apiMocks = vi.hoisted(() => ({
  fetchSession: vi.fn(),
  fetchUsers: vi.fn(),
  searchUsers: vi.fn(),
  updateUserRole: vi.fn(),
}));

vi.mock("@/lib/api/auth", () => ({ fetchSession: apiMocks.fetchSession }));
vi.mock("@/lib/api", () => ({
  fetchUsers: apiMocks.fetchUsers,
  searchUsers: apiMocks.searchUsers,
  updateUserRole: apiMocks.updateUserRole,
}));
// Navbar pulls in useRouter through useLogout.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("@/lib/api/users", () => ({
  fetchMe: vi.fn().mockRejectedValue(new Error("offline")),
}));

import RolesPage from "./page";

describe("RolesPage authorization", () => {
  let container: HTMLDivElement;
  let root: Root;

  function setCachedUser(role: string) {
    useAuth.setState({
      user: { id: "u-1", username: "mallory", avatar_url: null, role },
    });
  }

  async function render() {
    await act(async () => {
      root.render(<RolesPage />);
    });
  }

  beforeEach(() => {
    apiMocks.fetchSession.mockReset().mockResolvedValue({ user_id: "u-1", role: "owner" });
    apiMocks.fetchUsers.mockReset().mockResolvedValue({ users: [], total: 0 });
    apiMocks.searchUsers.mockReset().mockResolvedValue({ users: [], total: 0 });
    apiMocks.updateUserRole.mockReset().mockResolvedValue({});
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    useAuth.setState({ user: null });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  it("does not render the role console or list users when /me contradicts a tampered owner role", async () => {
    setCachedUser("owner");
    apiMocks.fetchSession.mockResolvedValue({ user_id: "u-1", role: "user" });

    await render();

    expect(container.querySelector("h1")?.textContent).toBe("Owners only");
    expect(apiMocks.fetchUsers).not.toHaveBeenCalled();
  });

  it("renders the console and lists users once /me confirms the owner role", async () => {
    setCachedUser("owner");

    await render();

    expect(container.querySelector("h1")?.textContent).toBe("Manage roles");
    expect(apiMocks.fetchUsers).toHaveBeenCalledTimes(1);
  });

  it("does not list users for a signed-out caller", async () => {
    apiMocks.fetchSession.mockRejectedValue(new Error("no session"));

    await render();

    expect(container.textContent).toContain("Sign in to manage roles");
    expect(apiMocks.fetchUsers).not.toHaveBeenCalled();
  });

  it("does not list users for a cached admin who is not an owner", async () => {
    setCachedUser("admin");
    apiMocks.fetchSession.mockResolvedValue({ user_id: "u-1", role: "admin" });

    await render();

    expect(container.querySelector("h1")?.textContent).toBe("Owners only");
    expect(apiMocks.fetchUsers).not.toHaveBeenCalled();
  });
});