"use client";

import type { Category } from "@/lib/types";

interface CategoryBarProps {
  categories: Category[];
  selected: number | null;
  onSelect: (id: number | null) => void;
}

export default function CategoryBar({
  categories,
  selected,
  onSelect,
}: CategoryBarProps) {
  return (
    <div className="flex items-center gap-2 overflow-x-auto px-4 py-2">
      <button
        type="button"
        onClick={() => onSelect(null)}
        className={`shrink-0 rounded-full border px-3 py-1.5 text-sm font-medium transition-colors ${
          selected === null
            ? "border-rose-600 bg-rose-600 text-white"
            : "border-zinc-300 bg-white text-zinc-700 hover:border-zinc-400"
        }`}
      >
        All
      </button>
      {categories.map((c) => (
        <button
          key={c.id}
          type="button"
          onClick={() => onSelect(c.id)}
          className={`shrink-0 rounded-full border px-3 py-1.5 text-sm font-medium transition-colors ${
            selected === c.id
              ? "border-rose-600 bg-rose-600 text-white"
              : "border-zinc-300 bg-white text-zinc-700 hover:border-zinc-400"
          }`}
        >
          {c.name}
        </button>
      ))}
    </div>
  );
}