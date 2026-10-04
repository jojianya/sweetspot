// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useProfileEditor } from "./useProfileEditor";
import type { PublicProfile } from "@/lib/types";

const apiMocks = vi.hoisted(() => ({
  updateMyProfile: vi.fn(),
}));

vi.mock("@/lib/api", () => ({ updateMyProfile: apiMocks.updateMyProfile }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

import { useAuth } from "@/store/auth";

const profile: PublicProfile = {
  id: "u-1",
  username: "alice",
  avatar_url: null,
  socials: {},
  role: "user",
  created_at: "2026-01-01T00:00:00Z",
};

function EditorProbe() {
  const editor = useProfileEditor(profile, vi.fn());
  return (
    <>
      <span data-testid="save-error">{editor.saveError ?? "none"}</span>
      {/* Setting the username and saving are separate clicks on purpose: in one
          batched tick save() would still read the previous editUsername. */}
      <button
        type="button"
        data-testid="start-bad"
        onClick={() => {
          editor.setEditing(true);
          editor.setEditUsername("bad name");
        }}
      >
        start bad
      </button>
      <button
        type="button"
        data-testid="start-good"
        onClick={() => {
          editor.setEditing(true);
          editor.setEditUsername("alice_new");
        }}
      >
        start good
      </button>
      <button type="button" data-testid="save" onClick={() => void editor.save()}>
        save
      </button>
    </>
  );
}

describe("useProfileEditor username rule", () => {
  let container: HTMLDivElement;
  let root: Root;

  async function click(testId: string) {
    await act(async () => {
      (container.querySelector(`[data-testid="${testId}"]`) as HTMLButtonElement).click();
    });
  }

  function saveError(): string {
    return container.querySelector('[data-testid="save-error"]')?.textContent ?? "";
  }

  beforeEach(() => {
    apiMocks.updateMyProfile.mockReset().mockResolvedValue(profile);
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    useAuth.setState({ user: { ...profile } });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  // The server refuses a username with a space; the editor should say so
  // without spending a request.
  it("rejects an invalid rename before calling the API", async () => {
    await act(async () => {
      root.render(<EditorProbe />);
    });
    await click("start-bad");
    await click("save");

    expect(apiMocks.updateMyProfile).not.toHaveBeenCalled();
    expect(saveError()).toBe("Username may only contain letters, numbers, and _ . -");
  });

  it("still saves a valid rename", async () => {
    await act(async () => {
      root.render(<EditorProbe />);
    });
    await click("start-good");
    await click("save");

    expect(apiMocks.updateMyProfile).toHaveBeenCalledTimes(1);
    expect(saveError()).toBe("none");
  });
});