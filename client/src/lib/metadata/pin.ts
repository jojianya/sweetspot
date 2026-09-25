import type { Metadata } from "next";
import type { PinDetail } from "@/lib/types";

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

function isLoopbackHostname(hostname: string): boolean {
  const normalized = hostname.toLowerCase();
  if (normalized === "localhost" || normalized.endsWith(".localhost")) return true;
  if (normalized === "::1" || normalized === "[::1]") return true;
  return /^127(?:\.\d{1,3}){3}$/.test(normalized);
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
 * Converts API media values into crawler-safe absolute URLs. Relative values
 * and legacy localhost URLs are rooted at the public API because local storage
 * is served by the backend's /uploads route.
 */
export function resolvePublicMediaUrl(
  value: string | undefined,
  publicApiUrl: string | undefined
): string | null {
  const apiUrl = parseHttpUrl(publicApiUrl);
  const directUrl = parseHttpUrl(value);

  if (directUrl) {
    if (!isLoopbackHostname(directUrl.hostname)) return directUrl.toString();
    if (!apiUrl) return null;
    if (isLoopbackHostname(apiUrl.hostname)) return directUrl.toString();
    return new URL(`${directUrl.pathname}${directUrl.search}${directUrl.hash}`, apiUrl).toString();
  }

  if (!value || !apiUrl) return null;
  try {
    const resolved = new URL(value, apiUrl);
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
