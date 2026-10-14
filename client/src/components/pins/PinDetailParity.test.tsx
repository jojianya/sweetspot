// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Category } from "@/shared/types/category"
import type { PinDetail, PinPhoto } from "@/shared/types/pin";
import { useAuth } from "@/features/auth/store/auth";
import PinDetailPanel from "./PinDetailPanel";
import PinPageClient from "./PinPageClient";

// Characterization for the PinDetailPanel ↔ PinPageClient consolidation (C9).
// Every behavior the extraction must preserve, asserted identically on both
// surfaces except where the surfaces intentionally diverge (called out).
// Beyond DeleteParity (edit-button visibility) this covers: edit open/update
// props, delete destination, report visibility, view-on-map, sheet wiring,
// lightbox src, photo strip, save wiring, address copy, view merge, untitled.

const pushMock = vi.hoisted(() => vi.fn());
interface CapturedEdit {
  pin: PinDetail;
  categories: Category[];
  onClose: () => void;
  onUpdated: (pin: PinDetail) => void;
  onDeleted: (id: string) => void;
}
const captured = vi.hoisted((): {
  edit: CapturedEdit | null;
  collection: { pinId: string; onClose: () => void } | null;
  report: { pinId: string; onClose: () => void } | null;
  lightbox: { src: string } | null;
  comments: { pinId: string; compact?: boolean } | null;
} => ({
  edit: null,
  collection: null,
  report: null,
  lightbox: null,
  comments: null,
}));
const mockUsePinView = vi.hoisted(() => vi.fn());
const mockUsePinAddress = vi.hoisted(() => vi.fn());
const mockSaveState = vi.hoisted(() => ({
  saved: false,
  saving: false,
  error: null as string | null,
  handleSave: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("next/link", async () => {
  const { createElement } = await import("react");
  return {
    default: ({ href, children }: { href: string; children?: React.ReactNode }) =>
      createElement("a", { href }, children),
  };
});

vi.mock("@/shared/layout/Navbar", () => ({ default: () => null }));
vi.mock("@/hooks/usePinView", () => ({ usePinView: mockUsePinView }));
vi.mock("@/hooks/usePinAddress", () => ({ usePinAddress: mockUsePinAddress }));
vi.mock("@/features/collections/hooks/useOptimisticSave", () => ({ useOptimisticSave: () => mockSaveState }));
vi.mock("@/hooks/usePinShare", () => ({
  usePinShare: () => ({ copied: false, handleShare: vi.fn(), goDirections: vi.fn() }),
}));
vi.mock("@/features/social/hooks/useReaction", () => ({
  useReaction: () => ({ reacted: false, count: 0, busy: false, error: null, toggle: vi.fn() }),
}));
vi.mock("@/features/map/hooks/useCategories", () => ({ useCategories: () => ({ categories: [] }) }));
vi.mock("./PinEditSheet", () => ({
  default: (p: CapturedEdit) => {
    captured.edit = p;
    return (
      <div data-testid="edit-sheet">
        <button
          onClick={() => p.onUpdated({ ...p.pin, caption: "Updated caption", photos: [] })}
        >
          apply-update
        </button>
        <button onClick={() => p.onDeleted(p.pin.id)}>apply-delete</button>
      </div>
    );
  },
}));
vi.mock("@/features/collections/components/AddToCollectionSheet", () => ({
  default: (p: { pinId: string; onClose: () => void }) => {
    captured.collection = p;
    return <div data-testid="collection-sheet">collection-{p.pinId}</div>;
  },
}));
vi.mock("@/features/admin/components/ReportSheet", () => ({
  default: (p: { pinId: string; onClose: () => void }) => {
    captured.report = p;
    return <div data-testid="report-sheet">report-{p.pinId}</div>;
  },
}));
vi.mock("@/features/photos/components/PhotoLightbox", () => ({
  default: (p: {
    src: string;
    alt: string;
    count: number;
    index: number;
    onNavigate: (index: number) => void;
    onClose: () => void;
  }) => {
    captured.lightbox = p;
    return null;
  },
}));
vi.mock("@/features/comments/components/CommentsSection", () => ({
  default: (p: { pinId: string; compact?: boolean }) => {
    captured.comments = p;
    return (
      <div data-testid="comments-section">
        comments-{p.pinId}-{String(p.compact)}
      </div>
    );
  },
}));

function photo(id: string, base: string): PinPhoto {
  return {
    id,
    pin_id: "pin-1",
    photo_url: `/uploads/${base}.webp`,
    thumbnail_url: `/uploads/${base}-thumb.webp`,
    position: 0,
    created_at: "2026-01-02T00:00:00Z",
  };
}

function makePin(overrides: Partial<PinDetail> = {}): PinDetail {
  return {
    id: "pin-1",
    user_id: "owner-1",
    location: "POINT(0 0)",
    geohash: "s000",
    caption: "A great spot",
    category_id: 1,
    is_hidden: false,
    views: 42,
    good_spot_count: 0,
    created_at: "2026-01-02T00:00:00Z",
    category: "Food",
    username: "alice",
    avatar_url: null,
    photos: [photo("photo-1", "p1"), photo("photo-2", "p2")],
    ...overrides,
  };
}

function setViewer(role: "user" | "admin" | "owner" | null, id: string | null) {
  useAuth.setState({
    user:
      role === null || id === null
        ? null
        : { id, username: "viewer", avatar_url: null, role },
  });
}

function buttons(container: HTMLElement): HTMLButtonElement[] {
  return Array.from(container.querySelectorAll("button"));
}

function findButton(container: HTMLElement, text: string): HTMLButtonElement | undefined {
  return buttons(container).find((b) => b.textContent === text);
}

function editButton(container: HTMLElement): HTMLButtonElement | null {
  // Side panel labels it "Edit", the permalink page "Edit pin".
  return (
    (buttons(container).find(
      (b) => b.textContent === "Edit" || b.textContent === "Edit pin"
    ) as HTMLButtonElement) ?? null
  );
}

function heroImg(container: HTMLElement): HTMLImageElement | null {
  return container.querySelector('button[aria-label="View photo full screen"] img');
}

const surfaces = [
  [
    "side panel",
    (pin: PinDetail, h: { onDeleted: (id: string) => void }) => (
      <PinDetailPanel pin={pin} onClose={() => {}} onDeleted={h.onDeleted} />
    ),
  ],
  ["permalink page", (pin: PinDetail) => <PinPageClient initialPin={pin} />],
] as const;

describe.each(surfaces)("pin detail parity on %s", (_label, renderBody) => {
  let container: HTMLDivElement;
  let root: Root;
  const onDeleted = vi.fn();

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    onDeleted.mockClear();
    pushMock.mockClear();
    captured.edit = captured.collection = captured.report = captured.lightbox =
      captured.comments = null;
    mockUsePinView.mockReset().mockReturnValue(null);
    mockUsePinAddress.mockReset().mockReturnValue(null);
    mockSaveState.saved = false;
    mockSaveState.saving = false;
    mockSaveState.error = null;
    setViewer(null, null);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  async function render(pin: PinDetail) {
    await act(async () => {
      root.render(renderBody(pin, { onDeleted }));
    });
  }

  it("edit opens the sheet with the pin and the empty-categories fallback", async () => {
    setViewer("user", "owner-1");
    await render(makePin());
    editButton(container)?.click();
    await act(async () => {});
    expect(container.querySelector('[data-testid="edit-sheet"]')).not.toBeNull();
    expect(captured.edit?.pin.id).toBe("pin-1");
    // useCategories returns [] here, so both surfaces synthesize the pin's
    // own category for the sheet.
    expect(captured.edit?.categories).toEqual([{ id: 1, name: "Food", slug: "food" }]);
  });

  it("edit update applies the new caption", async () => {
    setViewer("user", "owner-1");
    await render(makePin());
    editButton(container)?.click();
    await act(async () => {});
    const apply = findButton(container, "apply-update");
    expect(apply).not.toBeUndefined();
    await act(async () => {
      apply?.click();
    });
    expect(container.textContent).toContain("Updated caption");
    expect(container.textContent).not.toContain("A great spot");
  });

  it("stranger sees Report, owner and logged-out visitor do not", async () => {
    setViewer("user", "stranger-2");
    await render(makePin());
    expect(findButton(container, "Report")).not.toBeUndefined();

    setViewer("user", "owner-1");
    await render(makePin());
    expect(findButton(container, "Report")).toBeUndefined();

    setViewer(null, null);
    await render(makePin());
    expect(findButton(container, "Report")).toBeUndefined();
  });

  it("add-to-collection opens the sheet with the pin id", async () => {
    setViewer("user", "stranger-2");
    await render(makePin());
    findButton(container, "Add to collection")?.click();
    await act(async () => {});
    expect(container.querySelector('[data-testid="collection-sheet"]')).not.toBeNull();
    expect(captured.collection?.pinId).toBe("pin-1");
  });

  it("report opens the sheet with the pin id", async () => {
    setViewer("user", "stranger-2");
    await render(makePin());
    findButton(container, "Report")?.click();
    await act(async () => {});
    expect(container.querySelector('[data-testid="report-sheet"]')).not.toBeNull();
    expect(captured.report?.pinId).toBe("pin-1");
  });

  it("hero click opens the lightbox with the hero src", async () => {
    setViewer(null, null);
    await render(makePin());
    const zoom = container.querySelector(
      'button[aria-label="View photo full screen"]'
    ) as HTMLButtonElement;
    expect(zoom).not.toBeNull();
    await act(async () => {
      zoom.click();
    });
    expect(captured.lightbox?.src).toContain("p1.webp");
  });

  it("photo strip switches the hero image", async () => {
    setViewer(null, null);
    await render(makePin());
    expect(heroImg(container)?.getAttribute("src")).toContain("p1.webp");
    const thumb2 = container.querySelector(
      'button[aria-label="Photo 2 of 2"]'
    ) as HTMLButtonElement;
    expect(thumb2).not.toBeNull();
    expect(thumb2.getAttribute("aria-pressed")).toBe("false");
    await act(async () => {
      thumb2.click();
    });
    expect(heroImg(container)?.getAttribute("src")).toContain("p2.webp");
    expect(
      container.querySelector('button[aria-label="Photo 2 of 2"]')?.getAttribute("aria-pressed")
    ).toBe("true");
  });

  it("save wiring renders saved state and error", async () => {
    mockSaveState.saved = true;
    mockSaveState.error = "Save failed";
    setViewer("user", "stranger-2");
    await render(makePin());
    expect(container.textContent).toContain("Saved");
    expect(container.textContent).toContain("Save failed");
  });

  it("resolved address renders on both surfaces", async () => {
    mockUsePinAddress.mockReturnValue("Portland, OR");
    setViewer(null, null);
    await render(makePin());
    expect(container.textContent).toContain("Portland, OR");
  });

  it("missing address with a point shows the finding copy on both", async () => {
    mockUsePinAddress.mockReturnValue(null);
    setViewer(null, null);
    await render(makePin());
    expect(container.textContent).toContain("Finding address…");
  });

  it("view merge replaces the initial count", async () => {
    mockUsePinView.mockReturnValue({ pinId: "pin-1", views: 7 });
    setViewer(null, null);
    await render(makePin());
    expect(container.textContent).toContain("7 views");
    expect(container.textContent).not.toContain("42 views");
  });

  it("null caption renders Untitled", async () => {
    setViewer(null, null);
    await render(makePin({ caption: null }));
    expect(container.textContent).toContain("Untitled");
  });
});

describe("pin detail divergent copy without a point", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    mockUsePinView.mockReset().mockReturnValue(null);
    mockUsePinAddress.mockReset().mockReturnValue(null);
    setViewer(null, null);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  it.each([
    ["side panel", "On the GoodSpot map"],
    ["permalink page", "Location pinned on the map"],
  ])("%s shows its own no-point copy", async (_label, copy) => {
    const pin = makePin({ location: "" });
    await act(async () => {
      root.render(
        _label === "side panel" ? (
          <PinDetailPanel pin={pin} onClose={() => {}} onDeleted={() => {}} />
        ) : (
          <PinPageClient initialPin={pin} />
        )
      );
    });
    expect(container.textContent).toContain(copy);
  });
});

describe("pin detail delete destination", () => {
  let container: HTMLDivElement;
  let root: Root;
  const onDeleted = vi.fn();

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    onDeleted.mockClear();
    pushMock.mockClear();
    mockUsePinView.mockReset().mockReturnValue(null);
    mockUsePinAddress.mockReset().mockReturnValue(null);
    setViewer("user", "owner-1");
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  it("panel delete calls onDeleted with the id", async () => {
    await act(async () => {
      root.render(<PinDetailPanel pin={makePin()} onClose={() => {}} onDeleted={onDeleted} />);
    });
    editButton(container)?.click();
    await act(async () => {});
    const applyDelete = findButton(container, "apply-delete");
    await act(async () => {
      applyDelete?.click();
    });
    expect(onDeleted).toHaveBeenCalledWith("pin-1");
  });

  it("page delete navigates home", async () => {
    await act(async () => {
      root.render(<PinPageClient initialPin={makePin()} />);
    });
    (editButton(container) as HTMLButtonElement)?.click();
    await act(async () => {});
    const applyDelete = findButton(container, "apply-delete");
    await act(async () => {
      applyDelete?.click();
    });
    expect(pushMock).toHaveBeenCalledWith("/");
  });
});

describe("permalink view-on-map link", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    mockUsePinView.mockReset().mockReturnValue(null);
    mockUsePinAddress.mockReset().mockReturnValue(null);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  async function renderPage() {
    await act(async () => {
      root.render(<PinPageClient initialPin={makePin()} />);
    });
  }

  function viewOnMap(): HTMLAnchorElement | null {
    return (
      Array.from(container.querySelectorAll("a")).find((a) =>
        a.textContent?.includes("View on map")
      ) ?? null
    );
  }

  it("owner's own pin links home", async () => {
    setViewer("user", "owner-1");
    await renderPage();
    expect(viewOnMap()?.getAttribute("href")).toBe("/");
  });

  it("stranger pin has no view-on-map link", async () => {
    setViewer("user", "stranger-2");
    await renderPage();
    expect(viewOnMap()).toBeNull();
  });

  it("moderator sees Edit but no view-on-map link", async () => {
    setViewer("admin", "moderator-9");
    await renderPage();
    expect(editButton(container)).not.toBeNull();
    expect(viewOnMap()).toBeNull();
  });
});

describe("comments placement parity", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    captured.comments = null;
    mockUsePinView.mockReset().mockReturnValue(null);
    mockUsePinAddress.mockReset().mockReturnValue(null);
    setViewer(null, null);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    useAuth.setState({ user: null });
  });

  it("panel toggles compact comments on Show/Hide", async () => {
    await act(async () => {
      root.render(<PinDetailPanel pin={makePin()} onClose={() => {}} onDeleted={() => {}} />);
    });
    expect(container.querySelector('[data-testid="comments-section"]')).toBeNull();
    // The toggle reads "Comments" + "Show" across two spans.
    const show = buttons(container).find((b) => b.textContent?.includes("Show"));
    expect(show).not.toBeUndefined();
    await act(async () => {
      show?.click();
    });
    expect(container.querySelector('[data-testid="comments-section"]')).not.toBeNull();
    expect(captured.comments).toMatchObject({ pinId: "pin-1", compact: true });
    expect(
      buttons(container).find((b) => b.textContent?.includes("Hide"))
    ).not.toBeUndefined();
  });

  it("page renders full comments inline with no toggle", async () => {
    await act(async () => {
      root.render(<PinPageClient initialPin={makePin()} />);
    });
    expect(container.querySelector('[data-testid="comments-section"]')).not.toBeNull();
    expect(captured.comments).toMatchObject({ pinId: "pin-1" });
    expect(captured.comments?.compact).toBeUndefined();
    expect(buttons(container).find((b) => b.textContent?.includes("Show"))).toBeUndefined();
  });
});
