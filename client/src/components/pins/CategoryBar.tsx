"use client";

import type { Category } from "@/lib/types";

interface CategoryBarProps {
  categories: Category[];
  selected: number | null;
  onSelect: (id: number | null) => void;
}

const DOT_COLORS = [
  "#e11d48",
  "#f97316",
  "#eab308",
  "#22c55e",
  "#0ea5e9",
  "#8b5cf6",
];

export default function CategoryBar({
  categories,
  selected,
  onSelect,
}: CategoryBarProps) {
  return (
    <div className="flex items-center gap-2 overflow-x-auto px-1 py-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      <button
        type="button"
        onClick={() => onSelect(null)}
        className={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-3.5 py-1.5 text-sm font-medium transition-all ${
          selected === null
            ? "border-rose-600/20 bg-rose-600 text-white shadow-md shadow-rose-600/25"
            : "border-white/60 bg-white/80 text-zinc-600 shadow-sm backdrop-blur hover:border-zinc-200 hover:bg-white hover:text-zinc-900"
        }`}
      >
        All
      </button>
      {categories.map((c) => {
        const active = selected === c.id;
        const dot = DOT_COLORS[c.id % DOT_COLORS.length];
        return (
          <button
            key={c.id}
            type="button"
            onClick={() => onSelect(c.id)}
            className={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-3.5 py-1.5 text-sm font-medium transition-all ${
              active
                ? "border-rose-600/20 bg-rose-600 text-white shadow-md shadow-rose-600/25"
                : "border-white/60 bg-white/80 text-zinc-600 shadow-sm backdrop-blur hover:border-zinc-200 hover:bg-white hover:text-zinc-900"
            }`}
          >
            <span
              className="h-2 w-2 rounded-full"
              style={{ backgroundColor: active ? "#ffffff" : dot }}
            />
            {c.name}
          </button>
        );
      })}
    </div>
  );
}