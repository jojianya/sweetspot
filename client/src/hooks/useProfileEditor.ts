"use client";

import { useEffect, useMemo, useState } from "react";
import { updateMyProfile } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import { checkUsername } from "@/lib/username";
import { buildProfileEdit, strSocial } from "@/lib/utils/profile";
import { useAuth } from "@/store/auth";
import type { PublicProfile } from "@/lib/types";

/**
 * Own-profile editor: form state, avatar preview with object-URL cleanup,
 * and diffed save (unchanged forms close without touching the API).
 * Extracted unchanged from app/users/[id]/page.tsx.
 */
export function useProfileEditor(
  profile: PublicProfile | null,
  onUpdated: (updated: PublicProfile) => void
) {
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
      // A rename is the only field with a character rule; check it here so the
      // user is told before the upload rather than after (mirrors the server,
      // see lib/username.ts).
      if (editUsername.trim() !== profile.username) {
        const usernameCheck = checkUsername(editUsername);
        if (!usernameCheck.ok) {
          setSaveError(usernameCheck.message);
          return;
        }
      }
      const edit = buildProfileEdit(profile, {
        username: editUsername,
        instagram: editInstagram,
        twitter: editTwitter,
        website: editWebsite,
        avatar: editAvatar,
      });
      if (!edit) {
        setEditing(false);
        return;
      }
      const updated = await updateMyProfile(edit);
      onUpdated(updated);
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

  return {
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
  };
}
