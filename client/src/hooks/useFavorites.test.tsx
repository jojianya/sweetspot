// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FavoriteEntry } from "@/lib/api";
import { useFavorites } from "./useFavorites";

const apiMocks = vi.hoisted(() => ({
  fetchFavorites: vi.fn(),
  fetchIsSaved: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function entry(id: string): FavoriteEntry {
  return {
    id,
    cover_url: "",
    caption: id,
    username: "alice",
    created_at: "2026-01-02T00:00:00Z",
    saved_at: "2026-01-02T00:00:00Z",
  } as FavoriteEntry;
}

/** Builds one API page answer: `entries.length` rows out of `total`. */
function page(entries: FavoriteEntry[], total: number) {
  return { entries, total };
}

function FavoritesProbe() {
  const f = useFavorites();
  return (
    <>
      <span data-testid="state">
        {f.loading ? "loading" : "settled"}:{f.loadingMore ? "more" : "nomore"}:
        {f.entries.length}:{f.total}:{f.error ?? "noerror"}:{f.hasMore ? "hasmore" : "nomore2"}
      </span>
      {f.hasMore && (
        <button type="button" data-testid="more" onClick={f.loadMore}>
          more
        </button>
      )}
      <button type="button" data-testid="retry" onClick={f.retry}>
        retry
      </button>
    </>
  );
}

describe("useFavorites", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    apiMocks.fetchFavorites.mockReset();
    apiMocks.fetchIsSaved.mockReset().mockResolvedValue(false);
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

  function text(): string {
    return container.querySelector('[data-testid="state"]')?.textContent ?? "";
  }

  function click(id: string) {
    return act(async () => {
      (container.querySelector(`[data-testid="${id}"]`) as HTMLButtonElement).click();
    });
  }

  // The bug this pins: the panel used to render entries.length as the saved
  // count, so a two-page account reported exactly one page and nothing else.
  it("keeps the server's total and the loaded page separate", async () => {
    apiMocks.fetchFavorites.mockResolvedValue(page([entry("p1"), entry("p2")], 5));
    await act(async () => {
      root.render(createElement(FavoritesProbe));
    });
    expect(text()).toBe("settled:nomore:2:5:noerror:hasmore");
  });

  it("asks for the maximum page size at offset 0 first", async () => {
    apiMocks.fetchFavorites.mockResolvedValue(page([], 0));
    await act(async () => {
      root.render(createElement(FavoritesProbe));
    });
    expect(apiMocks.fetchFavorites).toHaveBeenCalledWith({ limit: 200, offset: 0 });
  });

  it("appends the next page and advances the offset by what is loaded", async () => {
    apiMocks.fetchFavorites.mockResolvedValueOnce(page([entry("p1"), entry("p2")], 5));
    apiMocks.fetchFavorites.mockResolvedValueOnce(page([entry("p3")], 5));
    await act(async () => {
      root.render(createElement(FavoritesProbe));
    });
    await click("more");
    expect(apiMocks.fetchFavorites).toHaveBeenLastCalledWith({ limit: 200, offset: 2 });
    expect(text()).toBe("settled:nomore:3:5:noerror:hasmore");
  });

  it("stops offering load-more once the loaded count reaches the total", async () => {
    apiMocks.fetchFavorites.mockResolvedValue(page([entry("p1"), entry("p2")], 2));
    await act(async () => {
      root.render(createElement(FavoritesProbe));
    });
    expect(text()).toBe("settled:nomore:2:2:noerror:nomore2");
    expect(container.querySelector('[data-testid="more"]')).toBeNull();
  });

  it("guards against a second load-more while one is in flight", async () => {
    const pending = deferred<{ entries: FavoriteEntry[]; total: number }>();
    apiMocks.fetchFavorites.mockResolvedValueOnce(page([entry("p1")], 5));
    apiMocks.fetchFavorites.mockReturnValueOnce(pending.promise);
    await act(async () => {
      root.render(createElement(FavoritesProbe));
    });
    await click("more");
    expect(text()).toBe("settled:more:1:5:noerror:hasmore");
    // Still in flight: a second click must not start another request.
    await click("more");
    expect(apiMocks.fetchFavorites).toHaveBeenCalledTimes(2);
  });

  it("surfaces a load failure and lets retry restart from the first page", async () => {
    apiMocks.fetchFavorites.mockRejectedValueOnce(new Error("offline"));
    await act(async () => {
      root.render(createElement(FavoritesProbe));
    });
    expect(text()).toBe("settled:nomore:0:0:offline:nomore2");

    apiMocks.fetchFavorites.mockResolvedValue(page([entry("p1")], 1));
    await click("retry");
    expect(apiMocks.fetchFavorites).toHaveBeenLastCalledWith({ limit: 200, offset: 0 });
    expect(text()).toBe("settled:nomore:1:1:noerror:nomore2");
  });
});
