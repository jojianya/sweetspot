"use client";

import { fetchCategories } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { Category } from "@/lib/types";

export function useCategories() {
  const { data, error, retry } = useAsyncData<Category[]>(() => fetchCategories(), []);

  return { categories: data ?? [], error, retry };
}