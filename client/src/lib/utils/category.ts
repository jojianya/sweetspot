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
