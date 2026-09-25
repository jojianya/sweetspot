import { describe, expect, it } from "vitest";
import type { Category } from "@/lib/types";
import { categoryHref, categorySlug, resolveCategoryParam } from "./category";

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
