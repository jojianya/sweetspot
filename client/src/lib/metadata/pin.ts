import type { Metadata } from "next";
import type { PinDetail } from "@/lib/types";
import { extractUploadsSuffix, isUploadsPath } from "@/lib/uploads";

const SITE_NAME = "Goodspot";
const TITLE_MAX_LENGTH = 70;
const DESCRIPTION_MAX_LENGTH = 200;

function cleanText(value: string | null | undefined): string {
  return value?.trim().replace(/\s+/g, " ") ?? "";
}

function truncate(value: string, maxLength: number): string {
  const characters = Array.from(value);
  if (characters.length <= maxLength) return value;
  return `${characters.slice(0, Math.max(1, maxLength - 1)).join("")}…`;
}

function parseHttpUrl(value: string | undefined): URL | null {
  if (!value) return null;
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
}

/**
 * Converts API media values into crawler-safe absolute URLs. Any stored
 * /uploads URL — relative, localhost, or a stale LAN IP from a previous
 * STORAGE_BASE_URL — is rooted at the public API because local storage
 * is served by the backend's /uploads route. Non-uploads absolute URLs pass
 * through unchanged.
 *
 * Contract (for OG tags and links): returns string | null, null means omit.
 * Allowed: http/https only. Everything else (relative without apiUrl,
 * blob:, data:, javascript:, protocol-relative) is null. Uploads handling
 * shares isUploadsPath/extractUploadsSuffix with lib/media.ts.
 */
export function resolvePublicMediaUrl(
  value: string | undefined,
  publicApiUrl: string | undefined
): string | null {
  const trimmedValue = value?.trim();
  const trimmedApi = publicApiUrl?.trim();
  const apiUrl = parseHttpUrl(trimmedApi);
  const directUrl = parseHttpUrl(trimmedValue);

  if (directUrl) {
    if (!isUploadsPath(directUrl.pathname)) return directUrl.toString();
    if (!apiUrl) return null;
    return new URL(`${directUrl.pathname}${directUrl.search}${directUrl.hash}`, apiUrl).toString();
  }

  // Protocol-relative must not launder through the base into an evil host.
  if (trimmedValue?.startsWith("//")) return null;
  if (!trimmedValue || !apiUrl) return null;
  const uploads = extractUploadsSuffix(trimmedValue);
  if (uploads) {
    return new URL(uploads, apiUrl).toString();
  }
  try {
    const resolved = new URL(trimmedValue, apiUrl);
    return resolved.protocol === "http:" || resolved.protocol === "https:"
      ? resolved.toString()
      : null;
  } catch {
    return null;
  }
}

function pinTitle(pin: PinDetail): string {
  const caption = truncate(cleanText(pin.caption), TITLE_MAX_LENGTH);
  if (caption) return `${caption} · ${SITE_NAME}`;

  const category = truncate(cleanText(pin.category), 50);
  return category ? `${category} on ${SITE_NAME}` : `A spot on ${SITE_NAME}`;
}

function pinDescription(pin: PinDetail): string {
  const caption = truncate(cleanText(pin.caption), 150);
  const category = cleanText(pin.category);
  const username = cleanText(pin.username).replace(/^@+/, "");

  const description = caption
    ? `${caption}${username ? ` by @${username}` : ""}${category ? ` in ${category}` : ""} · Shared on ${SITE_NAME}.`
    : `${category || "Explore this spot"}${username ? ` shared by @${username}` : ""} on ${SITE_NAME}.`;

  return truncate(description, DESCRIPTION_MAX_LENGTH);
}

function imageAlt(pin: PinDetail): string {
  const caption = truncate(cleanText(pin.caption), 120);
  const category = cleanText(pin.category);
  const username = cleanText(pin.username);
  if (caption) {
    return `${caption}${category ? ` — ${category}` : ""}${username ? ` by @${username}` : ""}`;
  }
  return `${category || "Goodspot pin"}${username ? ` by @${username}` : ""}`;
}

export function buildPinMetadata(
  pin: PinDetail,
  siteUrl: URL,
  publicApiUrl: string | undefined
): Metadata {
  const canonicalUrl = new URL(`/pin/${encodeURIComponent(pin.id)}`, siteUrl).toString();
  const title = pinTitle(pin);
  const description = pinDescription(pin);
  const heroUrl = resolvePublicMediaUrl(pin.photos[0]?.photo_url, publicApiUrl);
  const image = heroUrl ? { url: heroUrl, alt: imageAlt(pin) } : null;

  return {
    title,
    description,
    alternates: { canonical: canonicalUrl },
    openGraph: {
      type: "website",
      url: canonicalUrl,
      siteName: SITE_NAME,
      title,
      description,
      ...(image ? { images: [image] } : {}),
    },
    twitter: {
      card: image ? "summary_large_image" : "summary",
      title,
      description,
      ...(image ? { images: [image] } : {}),
    },
  };
}
