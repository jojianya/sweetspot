import { describe, expect, it } from "vitest";
import { checkUsername, USERNAME_MAX_CHARS } from "./username";

// The client copy mirrors server/internal/modules/user/validate.go. If the two
// drift, the form either rejects a name the server would accept or accepts one
// it will not, so the rejects below are as important as the accepts.
describe("checkUsername", () => {
  const valid = [
    "alice",
    "Alice",
    "ALICE99",
    "user_name",
    "user.name",
    "user-name",
    "a.b_c-d",
    "123",
    "___",
    "a".repeat(USERNAME_MAX_CHARS),
  ];
  for (const name of valid) {
    it(`accepts ${JSON.stringify(name)}`, () => {
      expect(checkUsername(name)).toEqual({ ok: true });
    });
  }

  const invalid: Array<[string, string]> = [
    ["", "at least 3 characters"],
    ["ab", "at least 3 characters"],
    ["a".repeat(USERNAME_MAX_CHARS + 1), "at most 30 characters"],
    ["alice smith", "may only contain"],
    ["alice\tbob", "may only contain"],
    ["alice\nbob", "may only contain"],
    ["<script>", "may only contain"],
    ["alice<b>", "may only contain"],
    ['alice"', "may only contain"],
    ["alice'", "may only contain"],
    ["`alice`", "may only contain"],
    ["alice/bob", "may only contain"],
    ["alice\\bob", "may only contain"],
    ["alice@home", "may only contain"],
    ["alice:admin", "may only contain"],
    ["alice;drop", "may only contain"],
    ["../../etc", "may only contain"],
    ["%00alice", "may only contain"],
    // Unicode lookalikes: Cyrillic "а" for Latin "a", plus other non-ASCII.
    ["\u0430lice", "may only contain"],
    ["caf\u00e9", "may only contain"],
    ["\u7528\u6237\u540d", "may only contain"],
    ["emoji\u{1F600}name", "may only contain"],
    // An invisible character must not slip past the regex.
    ["alice\u200bname", "may only contain"],
  ];
  for (const [name, fragment] of invalid) {
    it(`rejects ${JSON.stringify(name)}`, () => {
      const result = checkUsername(name);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.message).toContain(fragment);
      }
    });
  }

  // Length is checked before characters, so a name that is both too short and
  // non-ASCII reports the length problem. The server checks in the same order,
  // which is what keeps the two messages in step.
  it("reports the length problem first, like the server", () => {
    const result = checkUsername("\u7528\u6237");
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.message).toContain("at least 3 characters");
    }
  });

  // Surrounding whitespace is trimmed before the rules run, exactly as the
  // server trims, so a padded-but-valid name is accepted rather than rejected
  // for a space the server would have removed.
  it("trims before checking", () => {
    expect(checkUsername("  alice  ")).toEqual({ ok: true });
  });

  // The server is the authority, so the client's message for a rejected
  // character must be the one the server sends; the server capitalizes nothing
  // and this string is compared against it in the API tests.
  it("uses the same wording as the server for a bad character", () => {
    const result = checkUsername("alice smith");
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.message).toBe("Username may only contain letters, numbers, and _ . -");
    }
  });
});