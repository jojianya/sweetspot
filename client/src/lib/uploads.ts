/**
 * Shared uploads-path handling for backend media URLs.
 *
 * The Go server stores absolute URLs (`{STORAGE_BASE_URL}/uploads/<id>.webp`)
 * at upload time. When the LAN IP changes every old URL points at a dead
 * host, so the client re-roots any `/uploads/...` pathname to same-origin
 * (see `lib/media.ts`) and metadata re-roots it to the public API
 * (see `lib/metadata/pin.ts`). Both use these helpers so the `/uploads/`
 * rule lives in exactly one place.
 */

/** True when the URL pathname is local storage served by the backend. */
export function isUploadsPath(pathname: string): boolean {
  return pathname.startsWith("/uploads/");
}

/**
 * Extracts the same-origin `/uploads/...` suffix (with query and hash
 * preserved) from any stored value — already-relative or absolute with any
 * host. Returns null when the value is not an uploads reference.
 */
export function extractUploadsSuffix(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  if (trimmed.startsWith("/uploads/")) return trimmed;
  if (trimmed.startsWith("uploads/")) return `/${trimmed}`;
  try {
    const url = new URL(trimmed);
    if (isUploadsPath(url.pathname)) {
      return `${url.pathname}${url.search}${url.hash}`;
    }
    return null;
  } catch {
    return null;
  }
}
