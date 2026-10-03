// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PublicProfile } from "@/lib/types";
import { useUsersAdmin, USERS_PAGE_SIZE } from "./useUsersAdmin";

const apiMocks = vi.hoisted(() => ({
  fetchUsers: vi.fn(),
  searchUsers: vi.fn(),
  updateUserRole: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

function user(id: string, username: string, role = "user"): PublicProfile {
  return {
    id,
    username,
    avatar_url: null,
    socials: {},
    role,
    created_at: "2026-01-01T00:00:00Z",
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function AdminProbe() {
  const admin = useUsersAdmin();
  return (
    <>
      <span data-testid="state">
        {admin.loading ? "loading" : "settled"}:{admin.loadingMore ? "more" : "nomore"}:
        {admin.results.length}:{admin.total}:{admin.activeQuery ?? "all"}:
        {admin.error ?? "noerror"}:{admin.notice ?? "nonotice"}:{admin.busyId ?? "free"}:
        {admin.isSearching ? "searching" : "listing"}:{admin.hasMore ? "hasmore" : "nomore2"}
      </span>
      <span data-testid="query">{admin.query}</span>
      <button type="button" data-testid="search" onClick={() => void admin.handleSearch({ preventDefault: () => {} } as React.FormEvent)}>
        search
      </button>
      <button type="button" data-testid="showall" onClick={admin.showAll}>
        showall
      </button>
      <button type="button" data-testid="more" onClick={admin.loadMore}>
        more
      </button>
      <button type="button" data-testid="retry" onClick={admin.retryLoad}>
        retry
      </button>
      <button
        type="button"
        data-testid="promote"
        onClick={() => void admin.handleRoleChange(user("u-1", "alice"), "admin")}
      >
        promote
      </button>
      <button type="button" data-testid="type" onClick={() => admin.setQuery("ali")}>
        type
      </button>
    </>
  );
}

describe("useUsersAdmin", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.fetchUsers
      .mockReset()
      .mockResolvedValue({ users: [user("u-1", "alice"), user("u-2", "bob")], total: 2 });
    apiMocks.searchUsers.mockReset().mockResolvedValue({ users: [user("u-1", "alice")], total: 1 });
    apiMocks.updateUserRole.mockReset().mockImplementation(async (id: string, role: string) => user(id, "alice", role));
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
  });

  function state(): string {
    return container.querySelector('[data-testid="state"]')?.textContent ?? "";
  }

  function click(testId: string): void {
    (container.querySelector(`[data-testid="${testId}"]`) as HTMLButtonElement).click();
  }

  async function renderProbe(): Promise<void> {
    await act(async () => {
      root.render(createElement(AdminProbe));
    });
  }

  it("loads the first page on mount", async () => {
    await renderProbe();
    expect(apiMocks.fetchUsers).toHaveBeenCalledWith({ limit: USERS_PAGE_SIZE, offset: 0 });
    expect(state()).toContain("settled:nomore:2:2:all:noerror:nonotice:free:listing:nomore2");
  });

  it("appends the next page on load more", async () => {
    apiMocks.fetchUsers
      .mockResolvedValueOnce({ users: [user("u-1", "alice")], total: 3 })
      .mockResolvedValueOnce({ users: [user("u-2", "bob"), user("u-3", "cid")], total: 3 });
    await renderProbe();
    await act(async () => {
      click("more");
    });
    expect(apiMocks.fetchUsers).toHaveBeenLastCalledWith({ limit: USERS_PAGE_SIZE, offset: 1 });
    expect(state()).toContain(":3:3:");
  });

  it("searches and restores the full list", async () => {
    await renderProbe();
    await act(async () => {
      click("type");
    });
    await act(async () => {
      click("search");
    });
    expect(apiMocks.searchUsers).toHaveBeenCalledWith("ali");
    expect(state()).toContain(":1:1:ali:");
    expect(state()).toContain(":searching:");
    await act(async () => {
      click("showall");
    });
    expect(state()).toContain(":all:");
    expect(state()).toContain(":listing:");
  });

  it("promotes a user with a notice and swaps the row", async () => {
    await renderProbe();
    await act(async () => {
      click("promote");
    });
    expect(apiMocks.updateUserRole).toHaveBeenCalledWith("u-1", "admin");
    expect(state()).toContain("@alice is now admin.");
    expect(state()).toContain(":free:");
  });

  it("surfaces role failures and frees the row", async () => {
    apiMocks.updateUserRole.mockRejectedValue(new Error("forbidden"));
    await renderProbe();
    await act(async () => {
      click("promote");
    });
    expect(state()).toContain("forbidden");
    expect(state()).toContain(":free:");
  });

  it("retries a failed initial load", async () => {
    apiMocks.fetchUsers.mockRejectedValueOnce(new Error("down"));
    await renderProbe();
    expect(state()).toContain("down");
    await act(async () => {
      click("retry");
    });
    expect(apiMocks.fetchUsers).toHaveBeenCalledTimes(2);
    expect(state()).toContain("noerror");
  });
});
