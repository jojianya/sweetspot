"use client";

import { useState } from "react";

/**
 * Share + directions for a pin detail. `handleShare` prefers the native share
 * sheet and falls back to the clipboard (with a transient "Copied" flag);
 * every failure path is swallowed so sharing never surfaces an error.
 * `goDirections` opens Google Maps for the parsed coordinate. Extracted
 * unchanged from PinDetailPanel and PinPageClient.
 */
export function usePinShare(
  caption: string | null,
  pinId: string,
  point: { lat: number; lng: number } | null
) {
  const [copied, setCopied] = useState(false);

  const handleShare = async () => {
    const text = `Check out ${caption ?? "this place"} on GoodSpot`;
    const url = `${window.location.origin}/pin/${pinId}`;
    if (navigator.share) {
      try {
        await navigator.share({ title: text, text, url });
        return;
      } catch {
        // user dismissed the share sheet; fall through to clipboard
      }
    }
    try {
      await navigator.clipboard.writeText(`${text} ${url}`);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore clipboard failures
    }
  };

  const goDirections = () => {
    if (!point) return;
    window.open(
      `https://www.google.com/maps/dir/?api=1&destination=${point.lat},${point.lng}`,
      "_blank",
      "noopener,noreferrer"
    );
  };

  return { copied, handleShare, goDirections };
}
