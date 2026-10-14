// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Comment } from "../types";

// One mutable feed stands in for useComments. Both describes below drive it
// from their own beforeEach, which is what lets the two subjects share a
// harness without either seeing the other's comment list.
const feed = vi.hoisted(() => ({
  comments: [] as Comment[],
  loading: false,
  error: null as string | null,
  retry: vi.fn(),
  add: vi.fn(),
  remove: vi.fn(),
}));

vi.mock("../hooks/useComments", () => ({
  useComments: () => feed,
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

/** The single visible comment the delete-failure subject clicks on. */
function singleComment(): Comment {
  return {
    id: "c-1",
    pin_id: "pin-1",
    user_id: "u-1",
    body: "nice spot",
    is_hidden: false,
    created_at: "2026-01-01T00:00:00Z",
    username: "alice",
    avatar_url: null,
  };
}

/** A comment whose author may or may not still exist. */
function orphanComment(userId: string | null, username: string | null): Comment {
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

import { useAuth } from "@/features/auth/store/auth";
import CommentsSection from "./CommentsSection";

describe("CommentsSection delete failure", () => {
  let container: HTMLDivElement;
  let root: Root;

  function alerts(): string[] {
    return Array.from(container.querySelectorAll('[role="alert"]')).map(
      (n) => n.textContent ?? ""
    );
  }

  function deleteButton(): HTMLButtonElement {
    return container.querySelector('[aria-label="Delete comment"]') as HTMLButtonElement;
  }

  beforeEach(() => {
    feed.remove.mockReset().mockResolvedValue(undefined);
    feed.add.mockReset().mockResolvedValue({} as Comment);
    feed.comments = [singleComment()];
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    useAuth.setState({
      user: { id: "u-1", username: "alice", avatar_url: null, role: "user" },
    });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  async function render() {
    await act(async () => {
      root.render(<CommentsSection pinId="pin-1" />);
    });
  }

  // A failed delete leaves the comment in place; saying nothing made the user
  // believe it was gone.
  it("surfaces a delete failure instead of swallowing it", async () => {
    feed.remove.mockRejectedValue(new Error("you can only delete your own comments"));
    await render();
    expect(alerts()).toEqual([]);

    await act(async () => {
      deleteButton().click();
    });

    expect(feed.remove).toHaveBeenCalledWith("c-1");
    expect(alerts()).toContain("you can only delete your own comments");
  });

  it("shows no error when the delete succeeds", async () => {
    await render();

    await act(async () => {
      deleteButton().click();
    });

    expect(alerts()).toEqual([]);
  });

  it("names the comment box", async () => {
    await render();
    const box = container.querySelector("textarea") as HTMLTextAreaElement;
    expect(box?.getAttribute("aria-label")).toBeTruthy();
  });

  it("clears a previous delete error on the next attempt", async () => {
    feed.remove.mockRejectedValueOnce(new Error("network unavailable"));
    await render();

    await act(async () => {
      deleteButton().click();
    });
    expect(alerts()).toHaveLength(1);

    feed.remove.mockResolvedValueOnce(undefined);
    await act(async () => {
      deleteButton().click();
    });
    expect(alerts()).toEqual([]);
  });
});

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
    feed.comments = [
      orphanComment("user-1", "alice"),
      orphanComment(null, null),
      orphanComment("user-2", "bob"),
    ];
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  async function render() {
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
