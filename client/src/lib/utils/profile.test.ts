import { describe, expect, it } from "vitest";
import { buildProfileEdit, socialLinks, socialsChanged, strSocial } from "./profile";
import type { PublicProfile } from "@/lib/types";

function profile(overrides: Partial<PublicProfile> = {}): PublicProfile {
  return {
    id: "user-1",
    username: "alice",
    avatar_url: null,
    socials: { instagram: "alice", twitter: "", website: "alice.example" },
    role: "user",
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("strSocial", () => {
  it("passes strings through and coerces anything else to empty", () => {
    expect(strSocial("alice")).toBe("alice");
    expect(strSocial(undefined)).toBe("");
    expect(strSocial(null)).toBe("");
    expect(strSocial(42)).toBe("");
  });
});

describe("socialsChanged", () => {
  it("returns undefined when every edit matches the current value", () => {
    expect(
      socialsChanged(
        { instagram: "alice", twitter: "", website: "alice.example" },
        { instagram: "alice", twitter: "", website: "alice.example" }
      )
    ).toBeUndefined();
  });

  it("keeps unknown current keys and reports only the diff", () => {
    expect(
      socialsChanged(
        { instagram: "alice", extra: "kept" },
        { instagram: "alice2", twitter: "" }
      )
    ).toEqual({ instagram: "alice2", extra: "kept" });
  });

  it("treats a missing current key as empty", () => {
    expect(socialsChanged({}, { instagram: "@bob" })).toEqual({ instagram: "@bob" });
  });
});

describe("socialLinks", () => {
  it("builds handle links, strips @, and adds https to bare websites", () => {
    expect(
      socialLinks({ instagram: "@alice", twitter: "alice ", website: "alice.example" })
    ).toEqual([
      { label: "Instagram", href: "https://instagram.com/alice" },
      { label: "Twitter", href: "https://x.com/alice" },
      { label: "Website", href: "https://alice.example" },
    ]);
  });

  it("keeps explicit schemes and skips blank or non-string values", () => {
    expect(
      socialLinks({ instagram: "  ", twitter: 42, website: "http://plain.example/a" })
    ).toEqual([{ label: "Website", href: "http://plain.example/a" }]);
  });

  it("returns no links for empty socials", () => {
    expect(socialLinks({})).toEqual([]);
  });
});

describe("buildProfileEdit", () => {
  const fields = {
    username: "alice",
    instagram: "alice",
    twitter: "",
    website: "alice.example",
    avatar: null,
  };

  it("returns null when nothing changed", () => {
    expect(buildProfileEdit(profile(), fields)).toBeNull();
  });

  it("picks up a trimmed username change", () => {
    expect(
      buildProfileEdit(profile(), { ...fields, username: "  alice2  " })
    ).toEqual({ username: "alice2" });
  });

  it("ignores username whitespace that matches after trimming", () => {
    expect(buildProfileEdit(profile(), { ...fields, username: "  alice " })).toBeNull();
  });

  it("sends only the changed socials", () => {
    expect(buildProfileEdit(profile(), { ...fields, twitter: "@alice" })).toEqual({
      socials: { instagram: "alice", twitter: "@alice", website: "alice.example" },
    });
  });

  it("sends an avatar-only edit", () => {
    const avatar = new File(["x"], "a.png", { type: "image/png" });
    expect(buildProfileEdit(profile(), { ...fields, avatar })).toEqual({ avatar });
  });
});
