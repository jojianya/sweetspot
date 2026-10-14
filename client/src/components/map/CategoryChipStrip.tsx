"use client";

import { ChipIcon } from "./CategoryIcons";
import type { Category } from "@/shared/types/category";

/** Category chip strip: "All" plus one chip per category. */
export function CategoryChipStrip({
  categories,
  selectedCategory,
  onSelectCategory,
}: {
  categories: Category[];
  selectedCategory: number | null;
  onSelectCategory: (id: number | null) => void;
}) {
  // Mobile: full-width second row; sm+: sits between search and the auth
  // cluster exactly as before. min-w-0 + sm:flex-1 preserve the original
  // flex-1 sizing semantics on desktop.
  return (
    <div className="order-3 flex w-full min-w-0 shrink-0 items-center gap-2 sm:order-none sm:w-auto sm:flex-1">
      <div className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
        {(() => {
          const active = selectedCategory === null;
          return (
            <button
              type="button"
              onClick={() => onSelectCategory(null)}
              aria-pressed={active}
              className={
                "flex h-9 shrink-0 items-center gap-1.5 rounded-full border px-2.5 text-[13px] font-medium transition-colors " +
                (active
                  ? "border-blue-500 bg-blue-50 text-blue-600 dark:border-blue-500 dark:bg-blue-950/60 dark:text-blue-400"
                  : "border-[#E0E0E0] bg-white text-zinc-800 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:bg-zinc-800")
              }
            >
              <ChipIcon name="All" />
              <span className="whitespace-nowrap">All</span>
            </button>
          );
        })()}
        {categories.map((c) => {
          const active = selectedCategory === c.id;
          return (
            <button
              key={c.id}
              type="button"
              onClick={() => onSelectCategory(active ? null : c.id)}
              aria-pressed={active}
              className={
                "flex h-9 shrink-0 items-center gap-1.5 rounded-full border px-2.5 text-[13px] font-medium transition-colors " +
                (active
                  ? "border-blue-500 bg-blue-50 text-blue-600 dark:border-blue-500 dark:bg-blue-950/60 dark:text-blue-400"
                  : "border-[#E0E0E0] bg-white text-zinc-800 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:bg-zinc-800")
              }
            >
              <ChipIcon name={c.name} />
              <span className="whitespace-nowrap">{c.name}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
