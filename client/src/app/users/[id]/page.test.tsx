// @vitest-environment jsdom
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PinListEntry, PublicProfile, User, UserStats } from "@/lib/types";
import { useAuth } from "@/store/auth";
import ProfilePage from "./page";

const apiMocks = vi.hoisted(() => ({
  fetchUser: vi.fn(),
  fetchUserPins: vi.fn(),
  fetchUserCollections: vi.fn(),
  fetchUserStats: vi.fn(),
  followUser: vi.fn(),
  unfollowUser: vi.fn(),
  updateMyProfile: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);
vi.mock("@/components/layout/Navbar", () => ({ default: () => null }));
vi.mock("next/link", async () => {
  const { createElement } = await import("react");
  type LinkProps = {
    href: string;
    children?: ReactNode;
    className?: string;
  };
  return {
    default: ({ href, children, className }: LinkProps) =>
      createElement("a", { href, className }, children),
  };
});

const USER_ID = "user-1";
const OTHER_USER_ID = "user-2";
const PIN_ID = "pin-1";

const profile: PublicProfile = {
  id: USER_ID,
  username: "alice",
  avatar_url: null,
  socials: {},
  role: "user",
  created_at: "2026-01-01T00:00:00Z",
};

const pin: PinListEntry = {
  id: PIN_ID,
  user_id: USER_ID,
  location: "POINT(0 0)",
  geohash: "s000",
  caption: "A great spot",
  category_id: 1,
  is_hidden: false,
  views: 4,
  created_at: "2026-01-02T00:00:00Z",
  cover_url: "https://media.example/pin.webp",
  username: "alice",
};

const viewer: User = {
  id: OTHER_USER_ID,
  email: "viewer@example.com",
  username: "viewer",
  avatar_url: null,
  socials: {},
  role: "user",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

function stats(overrides: Partial<UserStats> = {}): UserStats {
  return {
    followers: 0,
    following: 0,
    pins_count: 0,
    is_following: false,
    ...overrides,
  };
}

function button(container: HTMLElement, label: string): HTMLButtonElement | undefined {
  return Array.from(container.querySelectorAll("button")).find(
    (candidate) => candidate.textContent === label
  );
}

describe("ProfilePage", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    localStorage.clear();
    useAuth.setState({ user: null, token: null });
    apiMocks.fetchUser.mockReset().mockResolvedValue(profile);
    apiMocks.fetchUserPins.mockReset().mockResolvedValue([pin]);
    apiMocks.fetchUserCollections.mockReset().mockResolvedValue([]);
    apiMocks.fetchUserStats.mockReset().mockResolvedValue(stats());
    apiMocks.followUser.mockReset().mockResolvedValue(undefined);
    apiMocks.unfollowUser.mockReset().mockResolvedValue(undefined);
    apiMocks.updateMyProfile.mockReset();

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
    localStorage.clear();
  });

  async function renderProfile(id = USER_ID) {
    await act(async () => {
      root.render(createElement(ProfilePage, { params: Promise.resolve({ id }) }));
    });
  }

  it("shows current counts and edit controls on the viewer's own profile", async () => {
    useAuth.getState().setAuth({ ...viewer, id: USER_ID, username: "alice" }, "token");
    apiMocks.fetchUserStats.mockResolvedValue(
      stats({ followers: 12, following: 3, pins_count: 1 })
    );

    await renderProfile();

    expect(apiMocks.fetchUserStats).toHaveBeenCalledWith(USER_ID);
    const statistics = container.querySelector('[aria-label="Profile statistics"]');
    expect(statistics?.textContent).toContain("1pins");
    expect(statistics?.textContent).toContain("12followers");
    expect(statistics?.textContent).toContain("3following");
    expect(button(container, "Edit profile")).toBeDefined();
    expect(button(container, "Follow")).toBeUndefined();
    expect(container.querySelector(`a[href="/pin/${PIN_ID}"]`)).not.toBeNull();
    expect(container.querySelector('img[alt="A great spot"]')).not.toBeNull();
  });

  it("follows another user from the profile and updates the count", async () => {
    useAuth.getState().setAuth(viewer, "token");
    apiMocks.fetchUserStats.mockResolvedValue(stats({ followers: 4, pins_count: 1 }));

    await renderProfile();

    const followButton = button(container, "Follow");
    expect(followButton).toBeDefined();
    await act(async () => {
      followButton?.click();
    });

    expect(apiMocks.followUser).toHaveBeenCalledWith(USER_ID);
    expect(apiMocks.unfollowUser).not.toHaveBeenCalled();
    expect(button(container, "Following")).toBeDefined();
    const statistics = container.querySelector('[aria-label="Profile statistics"]');
    expect(statistics?.textContent).toContain("5followers");
  });

  it("renders a not-found state when the profile does not exist", async () => {
    apiMocks.fetchUser.mockRejectedValue(new Error("user not found"));

    await renderProfile("missing-user");

    expect(container.textContent).toContain("User not found");
    expect(button(container, "Retry")).toBeUndefined();
  });
});
