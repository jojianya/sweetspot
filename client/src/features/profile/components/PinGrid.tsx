"use client";

import Link from "next/link";
import { iconStroke } from "@/shared/ui/icons";
import type { PinListEntry } from "@/shared/types/pin";
import { resolveMediaUrl } from "@/shared/lib/media";

export function PinGrid({ pins }: { pins: PinListEntry[] }) {
  if (pins.length === 0) {
    return (
      <p className="rounded-xl border border-dashed border-zinc-300 py-10 text-center text-sm text-zinc-400 dark:border-zinc-700 dark:text-zinc-500">
        No pins yet.
      </p>
    );
  }
  return (
    <div className="grid grid-cols-3 gap-1.5 sm:gap-2">
      {pins.map((p) => (
        <Link
          key={p.id}
          href={`/pin/${p.id}`}
          className="group relative aspect-square overflow-hidden rounded-xl bg-zinc-100 dark:bg-zinc-900"
        >
          {p.cover_url ? (
            <img
              src={resolveMediaUrl(p.cover_url)}
              alt={p.caption?.trim() || "Pin"}
              loading="lazy"
              decoding="async"
              className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <span className="flex h-full w-full items-center justify-center text-zinc-300 dark:text-zinc-600">
              <svg className="h-6 w-6" {...iconStroke} aria-hidden>
                <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z" />
                <circle cx="12" cy="10" r="3" />
              </svg>
            </span>
          )}
        </Link>
      ))}
    </div>
  );
}
