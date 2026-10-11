// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Comment } from "@/lib/types";
import { useAuth } from "@/store/auth";

function makeComment(userId: string | null, username: string | null): Comment {
  return {
    id: `comment-${userId ?? "orphan"}`,
    pin_id: "pin-1",
    user_id: userId,
    body: "Test comment",
    is_hidden: false,
    created_at: "2024-01-15T10:00:00Z",
    username,
    avatar_url: null,
  };
}

const hookMocks = vi.hoisted(() => ({
  comments: [
    makeComment("user-1", "alice"),
    makeComment(null, null),
    makeComment("user-2", "bob"),
  ],
  loading: false,
  error: null,
  retry: vi.fn(),
  add: vi.fn(),
  remove: vi.fn(),
}));

vi.mock("@/hooks/useComments", () => ({
  useComments: () => hookMocks,
}));

describe("CommentsSection with nullable user_id", () => {
  let container: HTMLDivElement;
  let root: Root;

  function userLinks(): HTMLAnchorElement[] {
    return Array.from(container.querySelectorAll('a[href^="/users/"]')) as HTMLAnchorElement[];
  }

  beforeEach(() => {
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

  async function render() {
    const CommentsSection = (await import("./CommentsSection")).default;
    await act(async () => {
      root.render(<CommentsSection pinId="pin-1" />);
    });
  }

  it("renders all comments including orphan (null user_id)", async () => {
    await render();
    expect(container.textContent).toContain("alice");
    expect(container.textContent).toContain("bob");
    expect(container.textContent).toContain("deleted user");
  });

  it("does not render link to /users/null for orphan comment", async () => {
    await render();
    const links = userLinks();
    expect(links).toHaveLength(2);
  });

  it("shows [deleted user] label for orphan comment", async () => {
    await render();
    expect(container.textContent).toContain("deleted user");
  });
});