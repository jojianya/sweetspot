import { describe, expect, it } from "vitest";
import {
  checkPassword,
  passwordByteLength,
  PASSWORD_MAX_BYTES,
  PASSWORD_MIN_CHARS,
} from "./password";

// These mirror the server's table-driven cases in
// server/pkg/password/password_test.go. Both sides must agree on the boundary,
// or a user gets a client error the server disagrees with (or vice versa).

describe("passwordByteLength", () => {
  it("counts ASCII as one byte per character", () => {
    expect(passwordByteLength("abc")).toBe(3);
  });

  it("counts multi-byte runes at their UTF-8 width", () => {
    expect(passwordByteLength("漢")).toBe(3); // 3 bytes
    expect(passwordByteLength("é")).toBe(2); // 2 bytes
    expect(passwordByteLength("\u{1F600}")).toBe(4); // 4 bytes
  });

  it("differs from string length for non-ASCII input", () => {
    // This is the whole point: "漢漢漢".length is 3, but bcrypt sees 9 bytes.
    expect("漢漢漢".length).toBe(3);
    expect(passwordByteLength("漢漢漢")).toBe(9);
  });
});

describe("checkPassword", () => {
  const accepted: [string, string][] = [
    ["ascii typical", "password123"],
    ["ascii at the byte limit", "a".repeat(PASSWORD_MAX_BYTES)],
    ["ascii at the character minimum", "a".repeat(PASSWORD_MIN_CHARS)],
    ["24 three-byte runes at the limit", "漢".repeat(24)],
    ["36 two-byte runes at the limit", "é".repeat(36)],
    ["8 emoji within both limits", "\u{1F600}".repeat(8)],
  ];

  const rejected: [string, string][] = [
    ["ascii under the character minimum", "short"],
    ["empty", ""],
    ["ascii one byte over", "a".repeat(PASSWORD_MAX_BYTES + 1)],
    ["25 three-byte runes over the byte limit", "漢".repeat(25)],
    ["37 two-byte runes over the byte limit", "é".repeat(37)],
    ["72 two-byte runes well over", "é".repeat(72)],
    ["emoji over the byte limit", "\u{1F600}".repeat(20)],
  ];

  it.each(accepted)("accepts %s", (_name, pw) => {
    expect(checkPassword(pw)).toEqual({ ok: true });
  });

  it.each(rejected)("rejects %s", (_name, pw) => {
    expect(checkPassword(pw).ok).toBe(false);
  });

  it("rejects a multi-byte password that is long in bytes but short in characters", () => {
    // 30 characters passes a naive `length >= 8` check, and passes a naive
    // `length <= 72` check too. Only a byte count catches it.
    const pw = "漢".repeat(30);
    expect(pw.length).toBeLessThanOrEqual(72);
    expect(checkPassword(pw).ok).toBe(false);
  });

  it("names the limit in the message so the user can act on it", () => {
    const result = checkPassword("漢".repeat(30));
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.message).toContain(String(PASSWORD_MAX_BYTES));
    }
  });

  it("names the minimum when the password is too short", () => {
    const result = checkPassword("short");
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.message).toContain(String(PASSWORD_MIN_CHARS));
    }
  });
});
