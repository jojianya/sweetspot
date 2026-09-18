"use client";

import { useEffect, useState } from "react";
import { getCurrentPosition, toGeoCoords, type GeoCoords } from "@/lib/utils";

export type { GeoCoords };

export function useGeolocation(): {
  position: GeoCoords | null;
  error: Error | null;
} {
  const [position, setPosition] = useState<GeoCoords | null>(null);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let cancelled = false;
    getCurrentPosition()
      .then((pos) => {
        if (!cancelled) setPosition(toGeoCoords(pos));
      })
      .catch((e: unknown) => {
        if (e instanceof Error && !cancelled) setError(e);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return { position, error };
}
