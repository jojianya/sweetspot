// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { UserStats } from "@/lib/types";
import { useFollow } from "./useFollow";

const apiMocks = vi.hoisted(() => ({
  fetchUserStats: vi.fn(),
  followUser: vi.fn(),
  unfollowUser: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

function stats(followers: number, isFollowing = false): UserStats {
  return { followers, following: 2, pins_count: 3, is_following: isFollowing };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function FollowProbe({ userId }: { userId: string }) {
  const { stats: userStats, busy, error, toggle } = useFollow(userId);
  return (
    <button
      type="button"
      disabled={busy || !userStats}
      onClick={() => void toggle()}
    >
      {userId}:{userStats?.followers ?? "none"}:{error ?? "none"}
    </button>
  );
}

describe("useFollow", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.fetchUserStats.mockReset();
    apiMocks.followUser.mockReset().mockResolvedValue(undefined);
    apiMocks.unfollowUser.mockReset().mockResolvedValue(undefined);

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

  it("rolls back an optimistic unfollow when the API fails", async () => {
    apiMocks.fetchUserStats.mockResolvedValue(stats(3, true));
    apiMocks.unfollowUser.mockRejectedValue(new Error("network unavailable"));

    await act(async () => {
      root.render(createElement(FollowProbe, { userId: "rollback" }));
    });
    expect(container.textContent).toBe("rollback:3:none");

    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });

    expect(apiMocks.unfollowUser).toHaveBeenCalledWith("rollback");
    expect(container.textContent).toBe("rollback:3:network unavailable");
  });

  it("does not expose stale stats after the profile target changes", async () => {
    const first = deferred<UserStats>();
    const second = deferred<UserStats>();
    apiMocks.fetchUserStats.mockImplementation((userId: string) =>
      userId === "first" ? first.promise : second.promise
    );

    await act(async () => {
      root.render(createElement(FollowProbe, { userId: "first" }));
    });
    await act(async () => {
      first.resolve(stats(1));
      await first.promise;
    });
    expect(container.textContent).toBe("first:1:none");

    await act(async () => {
      root.render(createElement(FollowProbe, { userId: "second" }));
    });
    expect(container.textContent).toBe("second:none:none");

    await act(async () => {
      second.resolve(stats(4));
      await second.promise;
    });
    expect(container.textContent).toBe("second:4:none");

    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(apiMocks.followUser).toHaveBeenCalledWith("second");
    expect(container.textContent).toBe("second:5:none");
  });
});
