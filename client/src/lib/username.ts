/**
 * Username rules, mirroring `server/internal/modules/user/validate.go`.
 *
 * The server is authoritative, but it only answers after a round trip. These
 * checks exist so the form can say what is wrong before submitting, and the
 * wording is kept identical so the two never disagree about why a name was
 * refused.
 *
 * Note what is deliberately *not* checked here: a user signing in, or loading
 * a profile, must never be blocked by these rules, because an account created
 * before the allowlist existed may still carry a username it rejects. This
 * module is for choosing a new username, nothing else.
 */

/** Shortest acceptable username, in characters. Mirrors the server's minimum. */
export const USERNAME_MIN_CHARS = 3;

/** Longest acceptable username, in characters. Mirrors the server's maximum. */
export const USERNAME_MAX_CHARS = 30;

/**
 * The only characters a username may contain. ASCII letters, digits, underscore,
 * dot and hyphen.
 *
 * An allowlist, not a denylist: the point is that nothing outside this set can
 * be stored, which rules out spaces, quotes, angle brackets, slashes and
 * non-ASCII letters — including ones that look exactly like ASCII, which is how
 * a homograph account impersonates a real one.
 */
const USERNAME_ALLOWED = /^[A-Za-z0-9_.-]+$/;

/**
 * Checks a username against the server's rules.
 *
 * Returns `null` when the name is acceptable, otherwise a message written to
 * match the server's exactly, so whichever answers first says the same thing.
 */
export function checkUsername(raw: string): { ok: true } | { ok: false; message: string } {
  const username = raw.trim();
  if (username.length < USERNAME_MIN_CHARS) {
    return { ok: false, message: `Username must be at least ${USERNAME_MIN_CHARS} characters` };
  }
  if (username.length > USERNAME_MAX_CHARS) {
    return { ok: false, message: `Username must be at most ${USERNAME_MAX_CHARS} characters` };
  }
  if (!USERNAME_ALLOWED.test(username)) {
    return { ok: false, message: "Username may only contain letters, numbers, and _ . -" };
  }
  return { ok: true };
}