"use client";

import { use, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import Avatar from "@/components/Avatar";
import { fetchUser, fetchUserCollections, fetchUserPins, updateMyProfile, type ProfileEdit } from "@/lib/api";
import { errorMessage, formatTime } from "@/lib/utils";
import { useFollow } from "@/hooks/useFollow";
import { useAuth } from "@/store/auth";
import type { CollectionEntry, PinListEntry, PublicProfile } from "@/lib/types";

interface ProfilePageProps {
  params: Promise<{ id: string }>;
}

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  viewBox: "0 0 24 24",
};

const INPUT_CLASS =
  "w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100";

function strSocial(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/** Builds the socials payload, dropping unchanged known keys so no-op saves are skipped. */
function socialsChanged(
  current: Record<string, unknown>,
  edits: Record<string, string>
): Record<string, unknown> | undefined {
  const next: Record<string, unknown> = { ...current };
  let changed = false;
  for (const key of Object.keys(edits)) {
    const before = typeof current[key] === "string" ? current[key] : "";
    if (edits[key] !== before) {
      next[key] = edits[key];
      changed = true;
    }
  }
  return changed ? next : undefined;
}

/** Renders profile socials as external links (instragram/twitter handles, website URL). */
function socialLinks(socials: Record<string, unknown>): Array<{ label: string; href: string }> {
  const links: Array<{ label: string; href: string }> = [];
  const handle = (v: string) => v.trim().replace(/^@/, "");
  if (typeof socials.instagram === "string" && socials.instagram.trim()) {
    links.push({ label: "Instagram", href: `https://instagram.com/${handle(socials.instagram)}` });
  }
  if (typeof socials.twitter === "string" && socials.twitter.trim()) {
    links.push({ label: "Twitter", href: `https://x.com/${handle(socials.twitter)}` });
  }
  if (typeof socials.website === "string" && socials.website.trim()) {
    const site = socials.website.trim();
    links.push({ label: "Website", href: /^https?:\/\//i.test(site) ? site : `https://${site}` });
  }
  return links;
}

function CollectionGlyph() {
  return (
    <svg className="h-4 w-4" {...stroke} aria-hidden>
      <path d="M8 6h12M8 12h12M8 18h12" />
      <path d="M3 6h.01M3 12h.01M3 18h.01" />
    </svg>
  );
}

function PinGrid({ pins }: { pins: PinListEntry[] }) {
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
              src={p.cover_url}
              alt={p.caption?.trim() || "Pin"}
              loading="lazy"
              decoding="async"
              className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <span className="flex h-full w-full items-center justify-center text-zinc-300 dark:text-zinc-600">
              <svg className="h-6 w-6" {...stroke} aria-hidden>
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

export default function ProfilePage({ params }: ProfilePageProps) {
  const { id } = use(params);
  const { token, user } = useAuth();
  const isSelf = user !== null && user.id === id;

  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [pins, setPins] = useState<PinListEntry[]>([]);
  const [collections, setCollections] = useState<CollectionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const { stats, busy, error: followError, toggle } = useFollow(id);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [prof, userPins, userCollections] = await Promise.all([
          fetchUser(id),
          fetchUserPins(id),
          fetchUserCollections(id),
        ]);
        if (cancelled) return;
        setProfile(prof);
        setPins(userPins);
        setCollections(userCollections);
      } catch (e) {
        if (cancelled) return;
        const message = errorMessage(e);
        if (/not found/i.test(message)) {
          setNotFound(true);
        } else {
          setError(message);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [id, attempt]);

  // Profile editing state (own profile only).
  const [editing, setEditing] = useState(false);
  const [editUsername, setEditUsername] = useState("");
  const [editInstagram, setEditInstagram] = useState("");
  const [editTwitter, setEditTwitter] = useState("");
  const [editWebsite, setEditWebsite] = useState("");
  const [editAvatar, setEditAvatar] = useState<File | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const avatarPreview = useMemo(
    () => (editAvatar ? URL.createObjectURL(editAvatar) : null),
    [editAvatar]
  );

  useEffect(() => {
    return () => {
      if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    };
  }, [avatarPreview]);

  const retry = () => {
    setError(null);
    setNotFound(false);
    setLoading(true);
    setAttempt((n) => n + 1);
  };

  const openEdit = () => {
    if (!profile) return;
    setEditUsername(profile.username);
    setEditInstagram(strSocial(profile.socials.instagram));
    setEditTwitter(strSocial(profile.socials.twitter));
    setEditWebsite(strSocial(profile.socials.website));
    setEditAvatar(null);
    setSaveError(null);
    setEditing(true);
  };

  const save = async () => {
    if (!profile) return;
    setSaving(true);
    setSaveError(null);
    try {
      const edit: ProfileEdit = {};
      const username = editUsername.trim();
      if (username !== profile.username) edit.username = username;
      const socials = socialsChanged(profile.socials, {
        instagram: editInstagram,
        twitter: editTwitter,
        website: editWebsite,
      });
      if (socials) edit.socials = socials;
      if (editAvatar) edit.avatar = editAvatar;
      if (edit.username === undefined && edit.socials === undefined && edit.avatar === undefined) {
        setEditing(false);
        return;
      }
      const updated = await updateMyProfile(edit);
      setProfile(updated);
      useAuth.setState((s) =>
        s.user
          ? {
              user: {
                ...s.user,
                username: updated.username,
                avatar_url: updated.avatar_url,
                socials: updated.socials,
              },
            }
          : s
      );
      setEditing(false);
      setEditAvatar(null);
    } catch (e) {
      setSaveError(errorMessage(e));
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <>
        <Navbar backHref="/" />
        <div className="flex flex-1 items-center justify-center text-sm text-zinc-400 dark:text-zinc-500">
          Loading profile…
        </div>
      </>
    );
  }

  if (notFound) {
    return (
      <>
        <Navbar backHref="/" />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
          <p className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            User not found
          </p>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            The profile you’re looking for doesn’t exist.
          </p>
          <Link
            href="/"
            className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
          >
            Back to map
          </Link>
        </div>
      </>
    );
  }

  if (error && !profile) {
    return (
      <>
        <Navbar backHref="/" />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
          <p className="text-sm text-rose-600 dark:text-rose-400">{error}</p>
          <button
            type="button"
            onClick={retry}
            className="rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Retry
          </button>
        </div>
      </>
    );
  }

  if (!profile) return null;

  const links = socialLinks(profile.socials);

  return (
    <>
      <Navbar backHref="/" />

      <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
        {/* Header */}
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
                  {stats?.pins_count ?? pins.length}
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
          {!isSelf && token && (
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
          {!isSelf && !token && (
            <Link
              href="/login"
              className="shrink-0 rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
            >
              Follow
            </Link>
          )}
        </div>

        {editing && (
          <section
            aria-label="Edit profile"
            className="mt-4 rounded-2xl border border-zinc-200/70 p-5 shadow-sm dark:border-zinc-800"
          >
            <div className="flex items-center gap-4">
              <Avatar
                src={avatarPreview ?? profile.avatar_url}
                username={editUsername || profile.username}
                className="h-16 w-16"
              />
              <div>
                <label className="inline-block cursor-pointer rounded-full border border-zinc-300 px-4 py-1.5 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800">
                  Change avatar
                  <input
                    type="file"
                    accept="image/jpeg,image/png"
                    className="sr-only"
                    onChange={(e) => setEditAvatar(e.target.files?.[0] ?? null)}
                  />
                </label>
                {editAvatar && (
                  <button
                    type="button"
                    onClick={() => setEditAvatar(null)}
                    className="mt-1 block text-xs text-zinc-500 hover:underline dark:text-zinc-400"
                  >
                    Remove selection
                  </button>
                )}
                <p className="mt-1 text-xs text-zinc-400 dark:text-zinc-500">
                  JPG or PNG, up to 5MB.
                </p>
              </div>
            </div>

            <div className="mt-5">
              <label
                htmlFor="edit-username"
                className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
              >
                Username
              </label>
              <input
                id="edit-username"
                type="text"
                value={editUsername}
                onChange={(e) => setEditUsername(e.target.value)}
                className={INPUT_CLASS}
              />
            </div>

            <div className="mt-4 grid gap-4 sm:grid-cols-3">
              <div>
                <label
                  htmlFor="edit-instagram"
                  className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  Instagram
                </label>
                <input
                  id="edit-instagram"
                  type="text"
                  value={editInstagram}
                  onChange={(e) => setEditInstagram(e.target.value)}
                  className={INPUT_CLASS}
                  placeholder="@handle"
                />
              </div>
              <div>
                <label
                  htmlFor="edit-twitter"
                  className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  Twitter
                </label>
                <input
                  id="edit-twitter"
                  type="text"
                  value={editTwitter}
                  onChange={(e) => setEditTwitter(e.target.value)}
                  className={INPUT_CLASS}
                  placeholder="@handle"
                />
              </div>
              <div>
                <label
                  htmlFor="edit-website"
                  className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
                >
                  Website
                </label>
                <input
                  id="edit-website"
                  type="text"
                  value={editWebsite}
                  onChange={(e) => setEditWebsite(e.target.value)}
                  className={INPUT_CLASS}
                  placeholder="https://…"
                />
              </div>
            </div>

            {saveError && (
              <p className="mt-3 text-sm text-rose-600 dark:text-rose-400" role="alert">
                {saveError}
              </p>
            )}

            <div className="mt-5 flex items-center justify-end gap-2">
              <button
                type="button"
                onClick={() => setEditing(false)}
                disabled={saving}
                className="rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={save}
                disabled={saving}
                className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-60"
              >
                {saving ? "Saving…" : "Save changes"}
              </button>
            </div>
          </section>
        )}

        {followError && (
          <p className="mt-3 text-sm text-rose-600 dark:text-rose-400" role="alert">
            {followError}
          </p>
        )}

        {/* Collections */}
        {collections.length > 0 && (
          <section aria-label="Collections" className="mt-8">
            <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-zinc-400 dark:text-zinc-500">
              <CollectionGlyph />
              Collections
            </h2>
            <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3">
              {collections.map((c) => (
                <li key={c.id}>
                  <Link
                    href={`/users/${id}?collection=${c.id}`}
                    className="flex items-center gap-2 rounded-xl border border-zinc-200/70 px-3 py-2.5 transition-colors hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800/60"
                  >
                    {c.cover_url ? (
                      <img
                        src={c.cover_url}
                        alt=""
                        className="h-9 w-9 shrink-0 rounded-lg object-cover"
                      />
                    ) : (
                      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600">
                        <CollectionGlyph />
                      </span>
                    )}
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                        {c.name}
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
        )}

        {/* Pins */}
        <section aria-label="Pins" className="mt-8">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-zinc-400 dark:text-zinc-500">
            Pins
          </h2>
          <PinGrid pins={pins} />
        </section>
      </div>
    </>
  );
}