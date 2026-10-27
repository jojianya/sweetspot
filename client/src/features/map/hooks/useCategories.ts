"use client";

import { fetchCategories } from "../api/categories";
import { useAsyncData } from "@/shared/hooks/useAsyncData";
import type { Category } from "@/shared/types/category";

export function useCategories() {
  const { data, error, retry } = useAsyncData<Category[]>(() => fetchCategories(), []);

  return { categories: data ?? [], error, retry };
}