"use client";

import Navbar from "@/shared/layout/Navbar";
import { Skeleton } from "@/shared/ui/Skeleton";
import { PageLoading } from "@/shared/ui/PageState";

/** Loading placeholder mirroring the profile page layout. */
export function ProfileSkeleton() {
  return (
    <>
      <Navbar backHref="/" />
      <PageLoading label="Loading profile…">
        <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
          {/* Header skeleton */}
          <div className="flex flex-col gap-5 rounded-2xl border border-zinc-200/70 p-5 shadow-sm dark:border-zinc-800 sm:flex-row sm:items-center">
            <Skeleton className="h-20 w-20 rounded-full shrink-0" />
            <div className="min-w-0 flex-1 space-y-3">
              <Skeleton className="h-7 w-48" />
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-4 w-24" />
              <div className="flex flex-wrap gap-x-4 gap-y-1">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-20" />
              </div>
            </div>
          </div>

          {/* Stats skeleton */}
          <dl aria-label="Profile statistics" className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-1 text-sm">
            <div className="flex items-baseline gap-1">
              <Skeleton className="h-5 w-12" />
              <Skeleton className="h-3 w-12" />
            </div>
            <div className="flex items-baseline gap-1">
              <Skeleton className="h-5 w-12" />
              <Skeleton className="h-3 w-16" />
            </div>
            <div className="flex items-baseline gap-1">
              <Skeleton className="h-5 w-12" />
              <Skeleton className="h-3 w-16" />
            </div>
          </dl>

          {/* Edit/Follow button skeleton */}
          <div className="mt-4 flex items-center gap-2">
            <Skeleton className="h-10 w-24 rounded-full" />
            <Skeleton className="h-10 w-24 rounded-full" />
          </div>

          {/* Collections skeleton */}
          <section aria-label="Collections" className="mt-8" aria-busy="true">
            <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-zinc-400 dark:text-zinc-500">
              <Skeleton className="h-4 w-4" />
              <Skeleton className="h-4 w-20" />
            </h2>
            <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3" role="list" aria-busy="true">
              {Array.from({ length: 3 }).map((_, i) => (
                <li key={`skeleton-${i}`}>
                  <Skeleton className="flex items-center gap-2 rounded-xl border border-zinc-200/70 p-3 transition-colors" aria-hidden="true">
                    <Skeleton className="h-9 w-9 shrink-0 rounded-lg" />
                    <span className="min-w-0 flex-1 space-y-1">
                      <Skeleton className="h-4 w-24" />
                      <Skeleton className="h-3 w-16" />
                    </span>
                  </Skeleton>
                </li>
              ))}
            </ul>
          </section>

          {/* Pins grid skeleton */}
          <section aria-label="Pins" className="mt-8" aria-busy="true">
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-400 dark:text-zinc-500">
              <Skeleton className="h-4 w-16" />
            </h2>
            <div className="grid grid-cols-3 gap-1.5 sm:gap-2" role="list" aria-busy="true">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={`skeleton-pin-${i}`} className="aspect-square rounded-xl" aria-hidden="true" />
              ))}
            </div>
          </section>
        </div>
      </PageLoading>
    </>
  );
}
