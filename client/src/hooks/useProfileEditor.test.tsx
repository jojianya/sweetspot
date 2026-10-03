// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuth } from "@/store/auth";
import type { PublicProfile } from "@/lib/types";
import { useProfileEditor } from "./useProfileEditor";

const apiMocks = vi.hoisted(() => ({
  updateMyProfile: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

const profile: PublicProfile = {
  id: "user-1",
  username: "alice",
  avatar_url: null,
  socials: { instagram: "alice", twitter: "", website: "alice.example" },
  role: "user",
  created_at: "2026-01-01T00:00:00Z",
};

function EditorProbe({
  onUpdated,
}: {
  onUpdated: (updated: PublicProfile) => void;
}) {
  const editor = useProfileEditor(profile, onUpdated);
  return (
    <>
      <span data-testid="state">
        {editor.editing ? "open" : "closed"}:{editor.editUsername}:{editor.editInstagram}:
        {editor.saveError ?? "noerror"}:{editor.saving ? "busy" : "idle"}
      </span>
      <button type="button" data-testid="open" onClick={editor.openEdit}>
        open
      </button>
      <button type="button" data-testid="save" onClick={() => void editor.save()}>
        save
      </button>
      <button
        type="button"
        data-testid="rename"
        onClick={() => editor.setEditUsername("alice2")}
      >
        rename
      </button>
    </>
  );
}

describe("useProfileEditor", () => {
  let container: HTMLDivElement;
  let root: Root;
  let updated: PublicProfile | null;

  beforeEach(() => {
    updated = null;
    apiMocks.updateMyProfile.mockReset();
    useAuth.setState({ user: null });
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
  });

  function state(): string {
    return container.querySelector('[data-testid="state"]')?.textContent ?? "";
  }

  function click(testId: string): void {
    (container.querySelector(`[data-testid="${testId}"]`) as HTMLButtonElement).click();
  }

  async function renderEditor(): Promise<void> {
    await act(async () => {
      root.render(
        createElement(EditorProbe, {
          onUpdated: (p) => {
            updated = p;
          },
        })
      );
    });
  }

  it("populates the form from the profile when opening", async () => {
    await renderEditor();
    await act(async () => {
      click("open");
    });
    expect(state()).toContain("open:alice:alice:");
  });

  it("closes without touching the API when nothing changed", async () => {
    await renderEditor();
    await act(async () => {
      click("open");
    });
    await act(async () => {
      click("save");
    });
    expect(apiMocks.updateMyProfile).not.toHaveBeenCalled();
    expect(state()).toContain("closed:");
  });

  it("saves a username change and syncs the store", async () => {
    const next = { ...profile, username: "alice2" };
    apiMocks.updateMyProfile.mockResolvedValue(next);
    await renderEditor();
    await act(async () => {
      click("open");
    });
    await act(async () => {
      click("rename");
    });
    await act(async () => {
      click("save");
    });
    expect(apiMocks.updateMyProfile).toHaveBeenCalledWith({ username: "alice2" });
    expect(updated).toEqual(next);
    expect(useAuth.getState().user?.username).toBe("alice2");
    expect(state()).toContain("closed:");
  });

  it("surfaces save failures and keeps the editor open", async () => {
    apiMocks.updateMyProfile.mockRejectedValue(new Error("name taken"));
    await renderEditor();
    await act(async () => {
      click("open");
    });
    await act(async () => {
      click("rename");
    });
    await act(async () => {
      click("save");
    });
    expect(state()).toContain("open:");
    expect(state()).toContain("name taken");
  });
});
