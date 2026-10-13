"use client";

import { resolveMediaUrl } from "@/lib/media";
import type { PinPhoto } from "@/lib/types";

/** Photo thumbnail buttons shared by the panel and permalink strips.
 * thumbBoxClass carries the shell's size and rounding (panel "h-16 w-16
 * rounded-xl", page "h-14 w-14 rounded-lg"); the wrapping scroller and any
 * section heading stay with each shell. Everything else is identical.
 */
export function PhotoThumbStrip({
  photos,
  index,
  onSelect,
  thumbBoxClass,
}: {
  photos: PinPhoto[];
  index: number;
  onSelect: (index: number) => void;
  thumbBoxClass: string;
}) {
  return (
    <>
      {photos.map((p, i) => {
        const thumbSrc =
          resolveMediaUrl(p.thumbnail_url) || resolveMediaUrl(p.photo_url);
        return (
          <button
            key={p.id}
            type="button"
            onClick={() => onSelect(i)}
            aria-label={`Photo ${i + 1} of ${photos.length}`}
            aria-pressed={i === index}
            className={`${thumbBoxClass} shrink-0 overflow-hidden border-2 transition-colors ${
              i === index ? "border-sky-500" : "border-transparent hover:border-zinc-300 dark:hover:border-zinc-600"
            }`}
          >
            {thumbSrc ? (
              <img
                src={thumbSrc}
                alt=""
                loading="lazy"
                decoding="async"
                className="h-full w-full object-cover"
              />
            ) : null}
          </button>
        );
      })}
    </>
  );
}
