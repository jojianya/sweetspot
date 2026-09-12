import api from "./client";
import type { Category } from "@/lib/types";

export async function fetchCategories(): Promise<Category[]> {
  const { data } = await api.get<Category[]>("/categories");
  return data;
}