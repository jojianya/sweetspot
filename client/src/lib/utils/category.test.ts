import { describe, expect, it } from "vitest";
import type { Category } from "@/lib/types";
import {
  canonicalCategoryAction,
  categoryHref,
  categorySlug,
  pickCategoryParam,
  resolveCategoryParam,
  slugForCategoryId,
} from "./category";

const categories: Category[] = [
  { id: 1, name: "Food", slug: "food" },
  { id: 2, name: "Live Music", slug: "live-music" },
];

describe("category URL parameters", () => {
  it("resolves a slug to the numeric category used by backend requests", () => {
    expect(resolveCategoryParam("food", categories)?.id).toBe(1);
  });

  it("treats an unresolvable slug as no category", () => {
    expect(resolveCategoryParam("not-a-category", categories)).toBeNull();
    expect(categoryHref("/", "category=not-a-category&zoom=10", null)).toBe("/?zoom=10");
  });

  it("canonicalizes a legacy numeric ID without clobbering other parameters", () => {
    const food = resolveCategoryParam("1", categories);

    expect(food?.slug).toBe("food");
    expect(categoryHref("/", "category=1&zoom=10", food?.slug ?? null)).toBe(
      "/?category=food&zoom=10"
    );
  });

  it("creates lowercase hyphenated slugs for fallback categories", () => {
    expect(categorySlug(" Live Music ")).toBe("live-music");
  });
});

describe("map category contract", () => {
  it("uses the initial category only when the query string is empty", () => {
    expect(pickCategoryParam("", null, "food")).toBe("food");
    expect(pickCategoryParam("category=food", "food", null)).toBe("food");
    expect(pickCategoryParam("zoom=10", null, "food")).toBeNull();
  });

  it("maps a selected id to its slug and null to cleared", () => {
    expect(slugForCategoryId(2, categories)).toBe("live-music");
    expect(slugForCategoryId(null, categories)).toBeNull();
    expect(slugForCategoryId(99, categories)).toBeNull();
  });

  it("clears a pending request once the URL echoes it back", () => {
    expect(canonicalCategoryAction("food", "food", categories)).toEqual({
      type: "clear-requested",
    });
    expect(canonicalCategoryAction("food", "1", categories)).toEqual({ type: "noop" });
    expect(canonicalCategoryAction(null, "food", categories)).toEqual({ type: "noop" });
    expect(canonicalCategoryAction(null, null, categories)).toEqual({
      type: "clear-requested",
    });
    expect(canonicalCategoryAction(undefined, "1", categories)).toEqual({
      type: "replace",
      slug: "food",
    });
  });

  it("canonicalizes legacy numeric and unknown values when idle", () => {
    expect(canonicalCategoryAction(undefined, "1", categories)).toEqual({
      type: "replace",
      slug: "food",
    });
    expect(canonicalCategoryAction(undefined, "nope", categories)).toEqual({
      type: "replace",
      slug: null,
    });
    expect(canonicalCategoryAction(undefined, "food", categories)).toEqual({
      type: "noop",
    });
    expect(canonicalCategoryAction(undefined, null, categories)).toEqual({ type: "noop" });
    expect(canonicalCategoryAction(undefined, "1", [])).toEqual({ type: "noop" });
  });
});
