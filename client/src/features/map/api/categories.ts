import api from "@/shared/api/client";
import type { Category } from "@/shared/types/category";

export async function fetchCategories(): Promise<Category[]> {
  const { data } = await api.get<Category[]>("/categories");
  return data;
}