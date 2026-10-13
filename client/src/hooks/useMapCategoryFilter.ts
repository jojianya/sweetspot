"use client";

import { useCallback, useEffect, useRef } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCategories } from "@/hooks/useCategories";
import {
  canonicalCategoryAction,
  categoryHref,
  pickCategoryParam,
  resolveCategoryParam,
  slugForCategoryId,
} from "@/lib/utils/category";

/**
 * Category filter synced with the URL. Category slugs are the public URL
 * contract; numeric IDs remain internal to the pin and realtime API requests.
 * Owns the requested-slug ref and the legacy-numeric canonicalization effect
 * so the map component only sees the resolved id and a select handler.
 */
export function useMapCategoryFilter(initialCategory: string | null = null) {
  const { categories, error: categoriesError, retry: retryCategories } = useCategories();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const requestedCategoryRef = useRef<{ slug: string | null } | null>(null);
  const currentQuery = searchParams.toString();
  const currentCategoryParam = searchParams.get("category");
  const categoryParam = pickCategoryParam(currentQuery, currentCategoryParam, initialCategory);
  const resolvedCategory =
    categories.length > 0 ? resolveCategoryParam(categoryParam, categories) : null;

  const effectiveCategory = resolvedCategory?.id ?? null;

  const replaceCategoryParam = useCallback(
    (slug: string | null) => {
      router.replace(categoryHref(pathname, currentQuery, slug), {
        scroll: false,
      });
    },
    [currentQuery, pathname, router]
  );

  const handleSelectCategory = useCallback(
    (id: number | null) => {
      const slug = slugForCategoryId(id, categories);
      requestedCategoryRef.current = { slug };
      replaceCategoryParam(slug);
    },
    [categories, replaceCategoryParam]
  );

  // Legacy numeric links resolve to their category and are canonicalized to
  // the slug. Unknown values remain "All" and are removed from the query.
  useEffect(() => {
    const requested = requestedCategoryRef.current;
    const action = canonicalCategoryAction(
      requested ? requested.slug : undefined,
      currentCategoryParam,
      categories
    );
    if (action.type === "clear-requested") {
      requestedCategoryRef.current = null;
      return;
    }
    if (action.type === "replace") {
      replaceCategoryParam(action.slug);
    }
  }, [categories, currentCategoryParam, replaceCategoryParam]);

  return { categories, categoriesError, retryCategories, effectiveCategory, handleSelectCategory };
}
