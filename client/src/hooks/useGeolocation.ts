"use client";

import { useEffect, useState } from "react";

export interface GeoCoords {
  lat: number;
  lng: number;
}

export function useGeolocation(): {
  position: GeoCoords | null;
  error: GeolocationPositionError | null;
} {
  const [position, setPosition] = useState<GeoCoords | null>(null);
  const [error, setError] = useState<GeolocationPositionError | null>(null);

  useEffect(() => {
    if (!("geolocation" in navigator)) return;
    navigator.geolocation.getCurrentPosition(
      (pos) =>
        setPosition({ lat: pos.coords.latitude, lng: pos.coords.longitude }),
      (err) => setError(err),
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 60000 }
    );
  }, []);

  return { position, error };
}