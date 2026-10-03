// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PageEmpty, PageError, PageLoading } from "./PageState";

const RETRY_CLASS =
  "rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800";

describe("PageState", () => {
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
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
  });

  async function render(node: React.ReactElement): Promise<void> {
    await act(async () => {
      root.render(node);
    });
  }

  describe("PageLoading", () => {
    it.each(["Loading feed…", "Loading reports…", "Loading profile…"])(
      "announces %s as a busy status with the skeleton inside",
      async (label) => {
        await render(
          createElement(PageLoading, { label }, createElement("ul", null, "rows"))
        );
        const region = container.querySelector('[role="status"]');
        expect(region?.getAttribute("aria-busy")).toBe("true");
        expect(region?.getAttribute("aria-label")).toBe(label);
        expect(region?.textContent).toContain(label);
        expect(region?.textContent).toContain("rows");
      }
    );
  });

  describe("PageError", () => {
    it("shows the message with an identically-styled Retry button", async () => {
      const onRetry = vi.fn();
      await render(createElement(PageError, { error: "server error", onRetry }));
      expect(container.textContent).toContain("server error");
      const button = container.querySelector("button");
      expect(button?.textContent).toBe("Retry");
      expect(button?.getAttribute("class")).toBe(RETRY_CLASS);
      await act(async () => {
        button?.click();
      });
      expect(onRetry).toHaveBeenCalledTimes(1);
    });

    it("carries no alert role today (feed, reports and profile alike)", async () => {
      await render(createElement(PageError, { error: "boom", onRetry: () => {} }));
      expect(container.querySelector('[role="alert"]')).toBeNull();
    });
  });

  describe("PageEmpty", () => {
    it("renders the feed copy with its centered description", async () => {
      await render(
        createElement(PageEmpty, {
          title: "Nothing here yet",
          description: createElement(
            "p",
            {
              className:
                "mx-auto mt-1 max-w-sm text-sm text-zinc-500 dark:text-zinc-400",
            },
            "Follow people from their profiles and their new pins will show up in this feed."
          ),
        })
      );
      const box = container.firstElementChild;
      expect(box?.getAttribute("class")).toContain("border-dashed");
      expect(box?.getAttribute("class")).toContain("py-12");
      expect(container.textContent).toContain("Nothing here yet");
      expect(container.querySelector("p + p")?.getAttribute("class")).toContain("max-w-sm");
    });

    it("renders the reports copy without the centering classes", async () => {
      await render(
        createElement(PageEmpty, {
          title: "No pending reports",
          description: createElement(
            "p",
            { className: "mt-1 text-sm text-zinc-500 dark:text-zinc-400" },
            "You're all caught up."
          ),
        })
      );
      expect(container.textContent).toContain("No pending reports");
      expect(container.querySelector("p + p")?.getAttribute("class")).not.toContain("max-w-sm");
    });
  });
});
