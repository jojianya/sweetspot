"use client";

import { Skeleton, SkeletonRegion } from "@/components/ui/Skeleton";

/** Loading rows for the saved/collections lists: thumbnail box plus two text lines. */
export function ListSkeleton({ label }: { label: string }) {
  return (
    <SkeletonRegion label={label}>
      <ul className="space-y-1" role="list" aria-busy="true">
        {Array.from({ length: 4 }).map((_, i) => (
          <li key={`skeleton-${i}`} className="flex items-center gap-3 rounded-xl px-2.5 py-2.5">
            <Skeleton className="h-14 w-14 shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1 space-y-1">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-3 w-1/2" />
            </div>
          </li>
        ))}
      </ul>
    </SkeletonRegion>
  );
}
