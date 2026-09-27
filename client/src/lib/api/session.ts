/**
 * Decides whether a 401 response means the browser's session has ended.
 *
 * The server only answers 401 for genuine session failures: the auth
 * middleware rejecting a missing, malformed, expired, or revoked token, and
 * the "account no longer exists" case in the pin handlers. Every one of those
 * means the httpOnly cookie can no longer authenticate anything, so the cached
 * user in the auth store is stale and must be dropped.
 *
 * The exception is the credential endpoints. A wrong password on
 * /auth/login is a 401 as well, but the caller has no session to lose there —
 * clearing on it would turn a typo into a logout.
 */
const CREDENTIAL_PATHS = ["/auth/login", "/auth/register"];

/**
 * Returns true when a 401 from `requestUrl` should clear the cached user.
 *
 * `requestUrl` is the request path as configured on the axios instance, so it
 * is relative ("/feed"), not absolute. An unknown or missing URL is treated as
 * not-a-session-failure: guessing wrong here would silently sign the user out.
 */
export function isSessionEnded(requestUrl: string | undefined): boolean {
  if (!requestUrl) return false;
  return !CREDENTIAL_PATHS.some((path) => requestUrl.includes(path));
}
