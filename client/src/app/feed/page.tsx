"use client";

import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import { ChevronRightIcon, FeedIcon, PinIcon } from "@/components/icons";
import { fetchFeed } from "@/lib/api";
import { relativeTime } from "@/lib/utils";
import { useAuth } from "@/store/auth";
import { useAsyncData } from "@/hooks/useAsyncData";
import { Skeleton } from "@/components/ui/Skeleton";
import { PageEmpty, PageError, PageLoading } from "@/components/ui/PageState";
import type { PinListEntry } from "@/lib/types";
import { resolveMediaUrl } from "@/lib/media";

export default function FeedPage() {
  const { user } = useAuth();
  const { data, loading, error, retry } = useAsyncData<PinListEntry[]>(
    () => fetchFeed(),
    [user],
    { enabled: !!user }
  );
  const pins = data ?? [];

  if (!user) {
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-4 px-4 text-center">
          <span className="flex items-center justify-center">
            <FeedIcon />
          </span>
          <div>
            <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
              Your feed is waiting
            </h1>
            <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
              Log in to see new pins from people you follow.
            </p>
          </div>
          <Link
            href="/login"
            className="rounded-full bg-rose-600 px-6 py-2.5 text-sm font-medium text-white hover:bg-rose-700"
          >
            Log in
          </Link>
        </div>
      </>
    );
  }

  return (
    <>
      <Navbar backHref="/" backLabel="Back to map" />

      <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
        <h1 className="mb-4 flex items-center gap-2 text-xl font-bold text-zinc-900 dark:text-zinc-100">
          <span className="text-rose-500">
            <FeedIcon />
          </span>
          Feed
        </h1>

        {loading && (
          <PageLoading label="Loading feed…">
            <ul className="space-y-2.5" role="list" aria-busy="true">
              {Array.from({ length: 5 }).map((_, i) => (
                <li key={`skeleton-${i}`}>
                  <Link
                    href="#"
                    className="flex items-center gap-3 rounded-xl border border-zinc-200/70 p-2.5"
                    aria-hidden="true"
                  >
                    <Skeleton className="h-16 w-16 shrink-0 rounded-lg" />
                    <span className="min-w-0 flex-1">
                      <Skeleton className="h-4 w-5/6" style={{ height: "16px" }} />
                      <Skeleton className="h-3 w-1/3" style={{ height: "12px" }} />
                    </span>
                    <Skeleton className="h-4 w-4 shrink-0" />
                  </Link>
                </li>
              ))}
            </ul>
          </PageLoading>
        )}

        {!loading && error && <PageError error={error} onRetry={retry} />}

        {!loading && !error && pins.length === 0 && (
          <PageEmpty
            title="Nothing here yet"
            description={
              <p className="mx-auto mt-1 max-w-sm text-sm text-zinc-500 dark:text-zinc-400">
                Follow people from their profiles and their new pins will show up
                in this feed.
              </p>
            }
          />
        )}

        <ul className="space-y-2.5">
          {pins.map((pin) => (
            <li key={pin.id}>
              <Link
                href={`/pin/${pin.id}`}
                className="flex items-center gap-3 rounded-xl border border-zinc-200/70 p-2.5 transition-colors hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800/60"
              >
                {(() => {
                  const cover = resolveMediaUrl(pin.cover_url);
                  return cover ? (
                    <img
                      src={cover}
                      alt=""
                      loading="lazy"
                      decoding="async"
                      className="h-16 w-16 shrink-0 rounded-lg object-cover"
                    />
                  ) : (
                    <span className="flex h-16 w-16 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600">
                      <PinIcon className="h-5 w-5" />
                    </span>
                  );
                })()}
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                    {pin.caption?.trim() || "Untitled spot"}
                  </span>
                  <span className="mt-0.5 flex items-center gap-2 text-xs text-zinc-500 dark:text-zinc-400">
                    {pin.username && (
                      <span className="font-medium text-sky-600 dark:text-sky-400">
                        @{pin.username}
                      </span>
                    )}
                    <span className="text-zinc-300 dark:text-zinc-600">·</span>
                    <time dateTime={pin.created_at}>{relativeTime(pin.created_at)}</time>
                  </span>
                </span>
                <ChevronRightIcon className="h-4 w-4 shrink-0 text-zinc-300 dark:text-zinc-600" />
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </>
  );
}