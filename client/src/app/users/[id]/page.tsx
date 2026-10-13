"use client";

import { use } from "react";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import { useFollow } from "@/hooks/useFollow";
import { useProfile } from "@/hooks/useProfile";
import { useProfileEditor } from "@/hooks/useProfileEditor";
import { useAuth } from "@/store/auth";
import { PageError } from "@/components/ui/PageState";
import { PinGrid } from "./_components/PinGrid";
import { ProfileCollections } from "./_components/ProfileCollections";
import { ProfileEditForm } from "./_components/ProfileEditForm";
import { ProfileHeader } from "./_components/ProfileHeader";
import { ProfileSkeleton } from "./_components/ProfileSkeleton";

interface ProfilePageProps {
  params: Promise<{ id: string }>;
}

export default function ProfilePage({ params }: ProfilePageProps) {
  const { id } = use(params);
  const { user } = useAuth();
  const isSelf = user !== null && user.id === id;

  const { profile, setProfile, pins, collections, loading, notFound, error, retry } =
    useProfile(id);

  const { stats, busy, error: followError, toggle } = useFollow(id);

  const {
    editing,
    setEditing,
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
    saving,
    saveError,
    avatarPreview,
    openEdit,
    save,
  } = useProfileEditor(profile, setProfile);

  if (loading) {
    return <ProfileSkeleton />;
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
          <PageError error={error} onRetry={retry} />
        </div>
      </>
    );
  }

  if (!profile) return null;

  return (
    <>
      <Navbar backHref="/" />

      <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
        <ProfileHeader
          profile={profile}
          isSelf={isSelf}
          user={user}
          stats={stats}
          pinsCount={pins.length}
          busy={busy}
          toggle={toggle}
          openEdit={openEdit}
        />

        {editing && (
          <ProfileEditForm
            profile={profile}
            editUsername={editUsername}
            setEditUsername={setEditUsername}
            editInstagram={editInstagram}
            setEditInstagram={setEditInstagram}
            editTwitter={editTwitter}
            setEditTwitter={setEditTwitter}
            editWebsite={editWebsite}
            setEditWebsite={setEditWebsite}
            editAvatar={editAvatar}
            setEditAvatar={setEditAvatar}
            avatarPreview={avatarPreview}
            saving={saving}
            saveError={saveError}
            save={save}
            onCancel={() => setEditing(false)}
          />
        )}

        {followError && (
          <p className="mt-3 text-sm text-rose-600 dark:text-rose-400" role="alert">
            {followError}
          </p>
        )}

        <ProfileCollections userId={id} collections={collections} />

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