"use client";

import { useEffect, useRef, useState } from "react";
import type { Category } from "@/lib/types";

interface CategoryDropdownProps {
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

export default function CategoryDropdown({
  categories,
  selected,
  onSelect,
}: CategoryDropdownProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handle = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handle);
    return () => document.removeEventListener("mousedown", handle);
  }, []);

  const active = categories.find((c) => c.id === selected) ?? null;

  const choose = (id: number | null) => {
    onSelect(id);
    setOpen(false);
  };

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex shrink-0 items-center gap-2 rounded-full border border-white/60 bg-white/90 px-4 py-3 text-sm font-medium text-zinc-700 shadow-lg shadow-zinc-900/5 backdrop-blur outline-none transition-colors hover:bg-white focus-visible:ring-2 focus-visible:ring-rose-600/20"
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span
          className="h-2 w-2 rounded-full"
          style={{
            backgroundColor: active
              ? DOT_COLORS[active.id % DOT_COLORS.length]
              : "#71717a",
          }}
        />
        <span className="whitespace-nowrap">{active ? active.name : "All categories"}</span>
        <svg
          className={"h-4 w-4 text-zinc-400 transition-transform" + (open ? " rotate-180" : "")}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="m6 9 6 6 6-6" />
        </svg>
      </button>

      {open && (
        <ul
          role="listbox"
          aria-label="Categories"
          className="absolute right-0 top-full z-30 mt-2 max-h-80 w-56 overflow-y-auto rounded-2xl border border-zinc-200/70 bg-white/95 py-1.5 shadow-xl shadow-zinc-900/10 backdrop-blur"
        >
          <li role="presentation">
            <button
              type="button"
              role="option"
              aria-selected={selected === null}
              onClick={() => choose(null)}
              className="flex w-full items-center gap-2.5 px-4 py-2 text-left text-sm text-zinc-700 hover:bg-zinc-50"
            >
              <span className="h-2 w-2 rounded-full bg-zinc-400" />
              <span className="flex-1">All categories</span>
              {selected === null && (
                <svg className="h-4 w-4 text-rose-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="m5 13 4 4L19 7" />
                </svg>
              )}
            </button>
          </li>
          {categories.map((c) => {
            const isActive = selected === c.id;
            return (
              <li key={c.id} role="presentation">
                <button
                  type="button"
                  role="option"
                  aria-selected={isActive}
                  onClick={() => choose(c.id)}
                  className="flex w-full items-center gap-2.5 px-4 py-2 text-left text-sm text-zinc-700 hover:bg-zinc-50"
                >
                  <span
                    className="h-2 w-2 rounded-full"
                    style={{ backgroundColor: DOT_COLORS[c.id % DOT_COLORS.length] }}
                  />
                  <span className="flex-1">{c.name}</span>
                  {isActive && (
                    <svg className="h-4 w-4 text-rose-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="m5 13 4 4L19 7" />
                    </svg>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}