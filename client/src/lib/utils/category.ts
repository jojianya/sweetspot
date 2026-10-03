import type { Category } from "@/lib/types";

/** Converts a display name into the URL shape used by category links. */
export function categorySlug(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

/**
 * Resolves the public category query value to its internal numeric ID.
 * Numeric values are accepted temporarily for links created before slugs.
 */
export function resolveCategoryParam(
  value: string | null,
  categories: readonly Category[]
): Category | null {
  const normalized = value?.trim().toLowerCase() ?? "";
  if (!normalized) return null;

  const bySlug = categories.find((category) => category.slug === normalized);
  if (bySlug) return bySlug;

  if (!/^\d+$/.test(normalized)) return null;
  const id = Number(normalized);
  if (!Number.isSafeInteger(id)) return null;

  return categories.find((category) => category.id === id) ?? null;
}

/** Replaces only the category parameter while preserving all other query values. */
export function categoryHref(pathname: string, search: string, slug: string | null): string {
  const params = new URLSearchParams(search);
  if (slug === null) {
    params.delete("category");
  } else {
    params.set("category", slug);
  }

  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

/**
 * Picks the raw category value for this render. An empty query string means
 * the route was opened without parameters, so the server-provided initial
 * category applies; otherwise the live query value wins.
 */
export function pickCategoryParam(
  currentQuery: string,
  currentParam: string | null,
  initialCategory: string | null
): string | null {
  return currentQuery === "" ? initialCategory : currentParam;
}

/** Maps a selected numeric category id to its public slug (null clears). */
export function slugForCategoryId(
  id: number | null,
  categories: readonly Category[]
): string | null {
  if (id === null) return null;
  return categories.find((category) => category.id === id)?.slug ?? null;
}

export type CanonicalCategoryAction =
  | { type: "clear-requested" }
  | { type: "replace"; slug: string | null }
  | { type: "noop" };

/**
 * Decides the canonicalization step for the category effect. A just-requested
 * slug (including null for "clear to All") takes precedence until the URL
 * echoes it back; only with no pending request do legacy numeric (or unknown)
 * values resolve to their slug, or to null ("All"). `undefined` means no
 * pending request; null means a pending clear.
 */
export function canonicalCategoryAction(
  requestedSlug: string | null | undefined,
  currentParam: string | null,
  categories: readonly Category[]
): CanonicalCategoryAction {
  if (requestedSlug !== undefined) {
    if (currentParam === requestedSlug) return { type: "clear-requested" };
    return { type: "noop" };
  }
  if (categories.length === 0 || currentParam === null) return { type: "noop" };
  const canonicalSlug = resolveCategoryParam(currentParam, categories)?.slug ?? null;
  if (canonicalSlug === currentParam) return { type: "noop" };
  return { type: "replace", slug: canonicalSlug };
}
