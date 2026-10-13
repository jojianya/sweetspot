"use client";

import Link from "next/link";
import { ListIcon } from "@/components/icons";
import { PinCoverImage } from "@/components/pins/PinCoverImage";
import type { CollectionEntry } from "@/lib/types";

export function ProfileCollections({
  userId,
  collections,
}: {
  userId: string;
  collections: CollectionEntry[];
}) {
  if (collections.length === 0) return null;
  return (
    <section aria-label="Collections" className="mt-8">
      <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-zinc-400 dark:text-zinc-500">
        <ListIcon />
        Collections
      </h2>
      <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3">
        {collections.map((c) => (
          <li key={c.id}>
            <Link
              href={`/users/${userId}?collection=${c.id}`}
              className="flex items-center gap-2 rounded-xl border border-zinc-200/70 px-3 py-2.5 transition-colors hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800/60"
            >
              <PinCoverImage
                url={c.cover_url}
                boxClass="h-9 w-9 rounded-lg"
                glyph={<ListIcon className="h-4 w-4" />}
              />
              <span className="min-w-0">
                <span className="block truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                  {c.name}
                  {c.is_private && (
                    <span className="ml-1.5 rounded-full bg-zinc-100 px-1.5 py-0.5 text-[10px] font-medium text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                      Private
                    </span>
                  )}
                </span>
                <span className="block text-xs text-zinc-500 dark:text-zinc-400">
                  {c.pin_count} pin{c.pin_count === 1 ? "" : "s"}
                </span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
