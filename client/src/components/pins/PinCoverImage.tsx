"use client";

import type { ReactNode } from "react";
import { resolveMediaUrl } from "@/lib/media";

/**
 * List-row cover thumbnail with a glyph fallback for pins without a cover.
 * Resolves the media URL itself, so callers pass the raw cover_url value.
 *
 * boxClass carries the size/shape (e.g. "h-14 w-14 rounded-xl"); positioning
 * (shrink-0) and object-fit are fixed. lazy and the fallback wash default to
 * the majority style; the trending row keeps its own flat wash and eager
 * thumbnails, and the add-to-collection row its eager thumbnails.
 */
export function PinCoverImage({
  url,
  boxClass,
  glyph,
  lazy = true,
  fallbackClassName = "bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600",
}: {
  url: string | null | undefined;
  boxClass: string;
  glyph?: ReactNode;
  lazy?: boolean;
  fallbackClassName?: string;
}) {
  const cover = resolveMediaUrl(url);
  if (!cover) {
    return (
      <span
        className={`flex shrink-0 items-center justify-center ${fallbackClassName} ${boxClass}`}
      >
        {glyph}
      </span>
    );
  }
  if (!lazy) {
    return <img src={cover} alt="" className={`shrink-0 object-cover ${boxClass}`} />;
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
