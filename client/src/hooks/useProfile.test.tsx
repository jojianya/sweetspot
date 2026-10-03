// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api/client";
import { useProfile } from "./useProfile";

const apiMocks = vi.hoisted(() => ({
  fetchUser: vi.fn(),
  fetchUserPins: vi.fn(),
  fetchUserCollections: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

function ProfileProbe({ id }: { id: string }) {
  const { profile, pins, collections, loading, notFound, error, retry } = useProfile(id);
  return (
    <>
      <span data-testid="state">
        {loading ? "loading" : "settled"}:{notFound ? "missing" : "found"}:{error ?? "noerror"}
      </span>
      <span data-testid="counts">
        {profile?.username ?? "noprofile"}:{pins.length}:{collections.length}
      </span>
      <button type="button" onClick={retry}>
        retry
      </button>
    </>
  );
}

describe("useProfile", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.fetchUser.mockReset().mockResolvedValue({ username: "alice" });
    apiMocks.fetchUserPins.mockReset().mockResolvedValue([{ id: "pin-1" }, { id: "pin-2" }]);
    apiMocks.fetchUserCollections.mockReset().mockResolvedValue([{ id: "col-1" }]);
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

  function counts(): string {
    return container.querySelector('[data-testid="counts"]')?.textContent ?? "";
  }

  it("fans out to profile, pins and collections in parallel", async () => {
    await act(async () => {
      root.render(createElement(ProfileProbe, { id: "user-1" }));
    });
    expect(apiMocks.fetchUser).toHaveBeenCalledWith("user-1");
    expect(apiMocks.fetchUserPins).toHaveBeenCalledWith("user-1");
    expect(apiMocks.fetchUserCollections).toHaveBeenCalledWith("user-1");
    expect(state()).toBe("settled:found:noerror");
    expect(counts()).toBe("alice:2:1");
  });

  it("maps a 404 to notFound", async () => {
    apiMocks.fetchUser.mockRejectedValue(new ApiError("user not found", 404));
    await act(async () => {
      root.render(createElement(ProfileProbe, { id: "missing" }));
    });
    expect(state()).toBe("settled:missing:noerror");
  });

  it("maps other failures to error and retries the load", async () => {
    apiMocks.fetchUser.mockRejectedValueOnce(new ApiError("server error", 500));
    apiMocks.fetchUser.mockResolvedValue({ username: "alice" });
    await act(async () => {
      root.render(createElement(ProfileProbe, { id: "user-1" }));
    });
    expect(state()).toBe("settled:found:server error");
    await act(async () => {
      (container.querySelector("button") as HTMLButtonElement).click();
    });
    expect(apiMocks.fetchUser).toHaveBeenCalledTimes(2);
    expect(state()).toBe("settled:found:noerror");
  });
});
