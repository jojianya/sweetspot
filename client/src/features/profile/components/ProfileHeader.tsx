"use client";

import Link from "next/link";
import Avatar from "@/shared/ui/Avatar";
import { formatTime } from "@/shared/utils";
import { socialLinks } from "../lib/profile";
import type { PersistedUser } from "@/store/auth";
import type { PublicProfile, UserStats } from "@/shared/types/social";

export function ProfileHeader({
  profile,
  isSelf,
  user,
  stats,
  pinsCount,
  busy,
  toggle,
  openEdit,
}: {
  profile: PublicProfile;
  isSelf: boolean;
  user: PersistedUser | null;
  stats: UserStats | null;
  pinsCount: number;
  busy: boolean;
  toggle: () => void;
  openEdit: () => void;
}) {
  const links = socialLinks(profile.socials);

  return (
    <div className="flex flex-col gap-5 rounded-2xl border border-zinc-200/70 p-5 shadow-sm dark:border-zinc-800 sm:flex-row sm:items-center">
      <Avatar
        src={profile.avatar_url}
        username={profile.username}
        className="h-20 w-20"
        fallbackClassName="bg-gradient-to-br from-rose-500 to-rose-700 text-white text-3xl"
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h1 className="truncate text-2xl font-bold text-zinc-900 dark:text-zinc-100">
            {profile.username}
          </h1>
          {isSelf && (
            <span className="shrink-0 rounded-full bg-zinc-100 px-2 py-0.5 text-xs font-medium text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
              you
            </span>
          )}
        </div>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Joined {formatTime(profile.created_at).split(",")[0]}
        </p>

        {links.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1">
            {links.map((link) => (
              <a
                key={link.label}
                href={link.href}
                target="_blank"
                rel="noreferrer"
                className="text-sm text-rose-600 hover:underline dark:text-rose-400"
              >
                {link.label}
              </a>
            ))}
          </div>
        )}

        {/* Stats */}
        <dl
          aria-label="Profile statistics"
          className="mt-3 flex flex-wrap items-center gap-x-6 gap-y-1 text-sm"
        >
          <div className="flex items-baseline gap-1">
            <dt className="sr-only">Pins</dt>
            <dd className="font-semibold text-zinc-900 dark:text-zinc-100">
              {stats?.pins_count ?? pinsCount}
            </dd>
            <span className="text-zinc-400">pins</span>
          </div>
          <div className="flex items-baseline gap-1">
            <dt className="sr-only">Followers</dt>
            <dd className="font-semibold text-zinc-900 dark:text-zinc-100">
              {stats?.followers ?? "—"}
            </dd>
            <span className="text-zinc-400">followers</span>
          </div>
          <div className="flex items-baseline gap-1">
            <dt className="sr-only">Following</dt>
            <dd className="font-semibold text-zinc-900 dark:text-zinc-100">
              {stats?.following ?? "—"}
            </dd>
            <span className="text-zinc-400">following</span>
          </div>
        </dl>
      </div>

      {isSelf && (
        <button
          type="button"
          onClick={openEdit}
          className="shrink-0 rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
        >
          Edit profile
        </button>
      )}
      {!isSelf && user && (
        <button
          type="button"
          onClick={toggle}
          disabled={busy || !stats}
          className={`shrink-0 rounded-full px-5 py-2 text-sm font-medium transition-colors disabled:opacity-60 ${
            stats?.is_following
              ? "border border-zinc-300 text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              : "bg-rose-600 text-white hover:bg-rose-700"
          }`}
        >
          {stats?.is_following ? "Following" : "Follow"}
        </button>
      )}
      {!isSelf && !user && (
        <Link
          href="/login"
          className="shrink-0 rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
        >
          Follow
        </Link>
      )}
    </div>
  );
}
