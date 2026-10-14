"use client";

import Avatar from "@/shared/ui/Avatar";
import { PHOTO_ACCEPT } from "@/features/photos";
import type { PublicProfile } from "@/shared/types/social";

const INPUT_CLASS =
  "w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100";

export function ProfileEditForm({
  profile,
  editUsername,
  setEditUsername,
  editInstagram,
  setEditInstagram,
  editTwitter,
  setEditTwitter,
  editWebsite,
  setEditWebsite,
  editAvatar,
  setEditAvatar,
  avatarPreview,
  saving,
  saveError,
  save,
  onCancel,
}: {
  profile: PublicProfile;
  editUsername: string;
  setEditUsername: (value: string) => void;
  editInstagram: string;
  setEditInstagram: (value: string) => void;
  editTwitter: string;
  setEditTwitter: (value: string) => void;
  editWebsite: string;
  setEditWebsite: (value: string) => void;
  editAvatar: File | null;
  setEditAvatar: (file: File | null) => void;
  avatarPreview: string | null;
  saving: boolean;
  saveError: string | null;
  save: () => void;
  onCancel: () => void;
}) {
  return (
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
              accept={PHOTO_ACCEPT}
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
          onClick={onCancel}
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
  );
}
