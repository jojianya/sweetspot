/**
 * Password length rules, mirroring the server's limits in
 * `server/pkg/password/password.go`.
 *
 * The two bounds are measured in different units, which is deliberate and is
 * the reason this module exists: bcrypt accepts at most 72 **bytes**, while a
 * sensible minimum is a number of **characters** whatever the alphabet. A
 * JavaScript string's `length` is UTF-16 code units, so it matches neither —
 * byte length has to be computed explicitly.
 */

/** Longest password bcrypt will accept, in bytes. Mirrors password.MaxLen. */
export const PASSWORD_MAX_BYTES = 72;

/** Shortest acceptable password, in characters. Mirrors password.MinRunes. */
export const PASSWORD_MIN_CHARS = 8;

/**
 * Byte length of a string as UTF-8.
 *
 * `TextEncoder` produces exactly the bytes bcrypt would see, so this is the
 * right measure rather than an estimate.
 */
export function passwordByteLength(value: string): number {
  return new TextEncoder().encode(value).length;
}

export type PasswordCheck =
  | { ok: true }
  | { ok: false; message: string };

/**
 * Validates a password against the server's rules.
 *
 * Deliberately mirrors the server rather than duplicating a looser version of
 * it: the client check is a convenience, the server is the authority, and the
 * two must agree on where the boundary sits.
 */
export function checkPassword(value: string): PasswordCheck {
  const chars = [...value].length;
  if (chars < PASSWORD_MIN_CHARS) {
    return {
      ok: false,
      message: `Password must be at least ${PASSWORD_MIN_CHARS} characters`,
    };
  }

  const bytes = passwordByteLength(value);
  if (bytes > PASSWORD_MAX_BYTES) {
    return {
      ok: false,
      message: `Password must be ${PASSWORD_MAX_BYTES} bytes or fewer`,
    };
  }

  return { ok: true };
}
