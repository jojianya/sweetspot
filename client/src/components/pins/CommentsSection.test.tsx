// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Comment } from "@/lib/types";

const hookMocks = vi.hoisted(() => ({
  remove: vi.fn(),
  add: vi.fn(),
  retry: vi.fn(),
}));

vi.mock("@/hooks/useComments", () => ({
  useComments: () => ({
    comments: [
      {
        id: "c-1",
        pin_id: "pin-1",
        user_id: "u-1",
        body: "nice spot",
        is_hidden: false,
        created_at: "2026-01-01T00:00:00Z",
        username: "alice",
        avatar_url: null,
      },
    ],
    loading: false,
    error: null,
    retry: hookMocks.retry,
    add: hookMocks.add,
    remove: hookMocks.remove,
  }),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

import { useAuth } from "@/store/auth";
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
    hookMocks.remove.mockReset().mockResolvedValue(undefined);
    hookMocks.add.mockReset().mockResolvedValue({} as Comment);
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
    hookMocks.remove.mockRejectedValue(new Error("you can only delete your own comments"));
    await render();
    expect(alerts()).toEqual([]);

    await act(async () => {
      deleteButton().click();
    });

    expect(hookMocks.remove).toHaveBeenCalledWith("c-1");
    expect(alerts()).toContain("you can only delete your own comments");
  });

  it("shows no error when the delete succeeds", async () => {
    await render();

    await act(async () => {
      deleteButton().click();
    });

    expect(alerts()).toEqual([]);
  });

  it("clears a previous delete error on the next attempt", async () => {
    hookMocks.remove.mockRejectedValueOnce(new Error("network unavailable"));
    await render();

    await act(async () => {
      deleteButton().click();
    });
    expect(alerts()).toHaveLength(1);

    hookMocks.remove.mockResolvedValueOnce(undefined);
    await act(async () => {
      deleteButton().click();
    });
    expect(alerts()).toEqual([]);
  });
});