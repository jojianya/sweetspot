"use client";

import type { ReactNode } from "react";
import { resolveMediaUrl } from "@/lib/media";

/**
 * List-row cover thumbnail with a glyph fallback for pins without a cover.
 * Resolves the media URL itself, so callers pass the raw cover_url value.
 *
 * boxClass carries the size/shape (e.g. "h-14 w-14 rounded-xl"); positioning
 * (shrink-0), object-fit and the fallback wash are fixed — every list row in
 * the app renders them identically.
 */
export function PinCoverImage({
  url,
  boxClass,
  glyph,
}: {
  url: string | null | undefined;
  boxClass: string;
  glyph?: ReactNode;
}) {
  const cover = resolveMediaUrl(url);
  if (!cover) {
    return (
      <span
        className={`flex shrink-0 items-center justify-center bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600 ${boxClass}`}
      >
        {glyph}
      </span>
    );
  }
  return (
    <img
      src={cover}
      alt=""
      loading="lazy"
      decoding="async"
      className={`shrink-0 object-cover ${boxClass}`}
    />
  );
}
