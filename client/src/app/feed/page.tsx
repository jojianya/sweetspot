"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import { fetchFeed } from "@/lib/api";
import { errorMessage, relativeTime } from "@/lib/utils";
import { useAuth } from "@/store/auth";
import type { PinListEntry } from "@/lib/types";

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  viewBox: "0 0 24 24",
};

function FeedIcon() {
  return (
    <svg className="h-5 w-5" {...stroke} aria-hidden>
      <path d="M4 19V5a2 2 0 0 1 2-2h13v16" />
      <path d="M4 19a2 2 0 0 0 2 2h13" />
      <path d="M9 7h6M9 11h6M9 15h4" />
    </svg>
  );
}

export default function FeedPage() {
  const { token } = useAuth();
  const [pins, setPins] = useState<PinListEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    fetchFeed()
      .then((data) => {
        if (!cancelled) setPins(data);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [attempt, token]);

  const retry = useCallback(() => {
    setError(null);
    setLoading(true);
    setAttempt((n) => n + 1);
  }, []);

  if (!token) {
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
          <p className="py-10 text-center text-sm text-zinc-400 dark:text-zinc-500">
            Loading pins…
          </p>
        )}

        {!loading && error && (
          <div className="flex flex-col items-center gap-3 py-10 text-center">
            <p className="text-sm text-rose-600 dark:text-rose-400">{error}</p>
            <button
              type="button"
              onClick={retry}
              className="rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Retry
            </button>
          </div>
        )}

        {!loading && !error && pins.length === 0 && (
          <div className="rounded-xl border border-dashed border-zinc-300 py-12 px-6 text-center dark:border-zinc-700">
            <p className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
              Nothing here yet
            </p>
            <p className="mx-auto mt-1 max-w-sm text-sm text-zinc-500 dark:text-zinc-400">
              Follow people from their profiles and their new pins will show up
              in this feed.
            </p>
          </div>
        )}

        <ul className="space-y-2.5">
          {pins.map((pin) => (
            <li key={pin.id}>
              <Link
                href={`/pin/${pin.id}`}
                className="flex items-center gap-3 rounded-xl border border-zinc-200/70 p-2.5 transition-colors hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800/60"
              >
                {pin.cover_url ? (
                  <img
                    src={pin.cover_url}
                    alt=""
                    loading="lazy"
                    decoding="async"
                    className="h-16 w-16 shrink-0 rounded-lg object-cover"
                  />
                ) : (
                  <span className="flex h-16 w-16 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600">
                    <svg className="h-5 w-5" {...stroke} aria-hidden>
                      <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z" />
                      <circle cx="12" cy="10" r="3" />
                    </svg>
                  </span>
                )}
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
                <svg
                  className="h-4 w-4 shrink-0 text-zinc-300 dark:text-zinc-600"
                  {...stroke}
                  aria-hidden
                >
                  <path d="m9 6 6 6-6 6" />
                </svg>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </>
  );
}