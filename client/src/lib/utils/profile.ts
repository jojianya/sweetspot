import type { ProfileEdit } from "@/lib/api";
import type { PublicProfile } from "@/lib/types";

export function strSocial(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/** Builds the socials payload, dropping unchanged known keys so no-op saves are skipped. */
export function socialsChanged(
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
export function socialLinks(socials: Record<string, unknown>): Array<{ label: string; href: string }> {
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

export interface ProfileEditFields {
  username: string;
  instagram: string;
  twitter: string;
  website: string;
  avatar: File | null;
}

/**
 * Diffs the edit form against the loaded profile. Returns null when nothing
 * changed so the caller can close the editor without touching the API.
 */
export function buildProfileEdit(
  profile: PublicProfile,
  fields: ProfileEditFields
): ProfileEdit | null {
  const edit: ProfileEdit = {};
  const username = fields.username.trim();
  if (username !== profile.username) edit.username = username;
  const socials = socialsChanged(profile.socials, {
    instagram: fields.instagram,
    twitter: fields.twitter,
    website: fields.website,
  });
  if (socials) edit.socials = socials;
  if (fields.avatar) edit.avatar = fields.avatar;
  if (edit.username === undefined && edit.socials === undefined && edit.avatar === undefined) {
    return null;
  }
  return edit;
}
