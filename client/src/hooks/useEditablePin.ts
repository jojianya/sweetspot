"use client";

import { useEffect, useMemo, useState } from "react";
import { resolveMediaUrl } from "@/shared/lib/media";
import { parsePoint } from "@/lib/utils/geo";
import { usePinView } from "@/hooks/usePinView";
import type { PinDetail } from "@/shared/types/pin";

/**
 * Local editable copy of a pin shared by the detail panel and the permalink
 * page: pin state plus the photo index, the counted view total merged in
 * once per fetch, the derived hero/lightbox sources and point, and the
 * edit-sheet update handler (which clamps the index to the new photo set).
 * The parent remounts via key={detail.id} (and route change on the permalink
 * page), so no pinId guard is needed here.
 */
export function useEditablePin(initialPin: PinDetail) {
  const [pin, setPin] = useState<PinDetail>(initialPin);
  const [index, setIndex] = useState(0);
  const view = usePinView(pin.id, pin.user_id);

  // Applies the counted total to local pin state once per fetch.
  useEffect(() => {
    if (!view) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- merge the freshly counted total into local pin state once per fetch
    setPin((prev) => ({ ...prev, views: view.views }));
  }, [view]);

  // The edit sheet hands back the updated pin (its response carries the
  // current photos), so there is nothing left to refetch.
  const handleUpdated = (updated: PinDetail) => {
    setPin(updated);
    setIndex((prev) => Math.min(prev, Math.max(0, updated.photos.length - 1)));
  };

  const photo = pin.photos[index];
  const count = pin.photos.length;
  const heroSrc = photo ? resolveMediaUrl(photo.photo_url) : "";
  const lightboxSrc = photo ? resolveMediaUrl(photo.photo_url) : "";
  const point = useMemo(() => parsePoint(pin.location), [pin.location]);
  const name = pin.caption?.trim();

  return {
    pin,
    index,
    setIndex,
    count,
    heroSrc,
    lightboxSrc,
    point,
    name,
    handleUpdated,
  };
}
