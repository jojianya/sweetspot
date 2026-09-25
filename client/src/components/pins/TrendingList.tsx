"use client";

import { useEffect, useRef } from "react";
import { CloseIcon } from "@/components/icons";
import type { TrendingPin } from "@/lib/types";

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  viewBox: "0 0 24 24",
};

export function TrendingIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg className={className} {...stroke} aria-hidden>
      <path d="M3 17l6-6 4 4 8-8" />
      <path d="M14 7h7v7" />
    </svg>
  );
}

interface TrendingListProps {
  pins: TrendingPin[];
  loading: boolean;
  error: string | null;
  onClose: () => void;
  onOpenPin: (pin: TrendingPin) => void;
}

/** Small "trending around here" list docked over the map for the current viewport. */
export default function TrendingList({
  pins,
  loading,
  error,
  onClose,
  onOpenPin,
}: TrendingListProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.tabIndex = -1;
    el.focus({ preventScroll: true });
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    el.addEventListener("keydown", onKey);
    return () => el.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      ref={ref}
      role="region"
      aria-label="Trending pins in this area"
      className="absolute bottom-24 left-4 z-10 w-72 overflow-hidden rounded-2xl bg-white/95 shadow-xl ring-1 ring-zinc-200/80 backdrop-blur dark:bg-zinc-900/95 dark:ring-zinc-700/80"
    >
      <header className="flex items-center justify-between border-b border-zinc-100 px-4 py-3 dark:border-zinc-800">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-zinc-900 dark:text-zinc-100">
          <TrendingIcon className="h-4 w-4 text-rose-500" />
          Trending here
        </h2>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close trending list"
          className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
        >
          <CloseIcon className="h-4 w-4" />
        </button>
      </header>

      {loading ? (
        <p className="px-4 py-5 text-xs text-zinc-500 dark:text-zinc-400">Loading…</p>
      ) : error ? (
        <p role="alert" className="px-4 py-5 text-xs text-rose-600 dark:text-rose-300">
          {error}
        </p>
      ) : pins.length === 0 ? (
        <p className="px-4 py-5 text-xs text-zinc-500 dark:text-zinc-400">
          Nothing trending in this view yet
        </p>
      ) : (
        <ul className="max-h-80 space-y-0.5 overflow-y-auto p-2">
          {pins.map((pin, i) => (
            <li key={pin.id}>
              <button
                type="button"
                onClick={() => onOpenPin(pin)}
                className="flex w-full items-center gap-3 rounded-xl px-2 py-2 text-left transition-colors hover:bg-zinc-100 dark:hover:bg-zinc-800"
              >
                <span className="w-5 shrink-0 text-center text-xs font-bold text-zinc-400 dark:text-zinc-500">
                  {i + 1}
                </span>
                {pin.cover_url ? (
                  <img
                    src={pin.cover_url}
                    alt=""
                    className="h-10 w-10 shrink-0 rounded-lg object-cover"
                  />
                ) : (
                  <div className="h-10 w-10 shrink-0 rounded-lg bg-zinc-200 dark:bg-zinc-800" />
                )}
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-zinc-900 dark:text-zinc-100">
                    {pin.caption?.trim() || "Untitled pin"}
                  </span>
                  <span className="block truncate text-xs text-zinc-500 dark:text-zinc-400">
                    {pin.username ?? "someone"} · {pin.views} views · {pin.comment_count} comments
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}