import { extractUploadsSuffix } from "./uploads";

/**
 * Normalizes backend media URLs to same-origin paths so images survive
 * STORAGE_BASE_URL changes (e.g. DHCP LAN-IP churn).
 *
 * The Go server stores absolute URLs (`{STORAGE_BASE_URL}/uploads/<id>.webp`)
 * in `pin_photos`/`users.avatar_url` at upload time. When the LAN IP changes
 * (e.g. .112 -> .107) every previously stored absolute URL points at a dead
 * host and `<img src={photo_url}>` breaks with no fallback. The same happens
 * for `localhost` URLs viewed from a phone (the phone's localhost is itself).
 *
 * All stored media lives under the backend's `/uploads` route (see
 * server/internal/http/router.go `r.Static("/uploads", ...)`), and
 * `next.config.ts` rewrites `/uploads/:path*` to the Go server, so the
 * browser can always load images same-origin.
 *
 * Contract (for `<img src>`): returns string, `""` means unusable.
 * Allowed: relative paths, `/uploads` rewrites (query/hash preserved),
 * http/https absolute URLs, `blob:` previews, `data:` URLs (tightened to
 * `data:image/*` in M5). Rejected (`""`): `javascript:`, protocol-relative
 * (`//host/...`), and any other scheme.
 */
export function resolveMediaUrl(value: string | null | undefined): string {
  if (typeof value !== "string") return "";
  const trimmed = value.trim();
  if (!trimmed) return "";
  if (trimmed.startsWith("//")) return "";

  const uploads = extractUploadsSuffix(trimmed);
  if (uploads) return uploads;

  try {
    const url = new URL(trimmed);
    if (url.protocol === "http:" || url.protocol === "https:") return trimmed;
    // Preserve pre-M5 behavior for blob:/data: previews; M5 narrows data:
    // to image/* explicitly. Anything else (javascript:, ftp:, ...) is rejected.
    if (url.protocol === "blob:" || url.protocol === "data:") return trimmed;
    return "";
  } catch {
    // Not an absolute URL: relative path, safe same-origin passthrough.
    return trimmed;
  }
}
