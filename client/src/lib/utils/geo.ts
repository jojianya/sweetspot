import type { LngLatBounds } from "maplibre-gl";

export function parsePoint(point: string): { lng: number; lat: number } | null {
  const m = point.match(/POINT\s*\(\s*(-?[\d.]+)\s+(-?[\d.]+)/i);
  if (!m) return null;
  const lng = parseFloat(m[1]);
  const lat = parseFloat(m[2]);
  if (
    Number.isNaN(lng) ||
    Number.isNaN(lat) ||
    lng < -180 ||
    lng > 180 ||
    lat < -90 ||
    lat > 90
  ) {
    return null;
  }
  return { lng, lat };
}

export type PinCoordinate = {
  id: string;
  lng: number;
  lat: number;
};

type LocatedPin = {
  id: string;
  location: string;
};

export const PIN_NEIGHBOR_RADIUS_METERS = 2_000;
export const PIN_NEIGHBOR_LIMIT = 8;

const EARTH_RADIUS_METERS = 6_371_000;

function toRadians(degrees: number): number {
  return (degrees * Math.PI) / 180;
}

/** Approximate great-circle distance between two WGS84 coordinates. */
export function distanceMeters(
  first: { lng: number; lat: number },
  second: { lng: number; lat: number }
): number {
  const latDelta = toRadians(second.lat - first.lat);
  const lngDelta = toRadians(second.lng - first.lng);
  const firstLat = toRadians(first.lat);
  const secondLat = toRadians(second.lat);
  const haversine =
    Math.sin(latDelta / 2) ** 2 +
    Math.cos(firstLat) * Math.cos(secondLat) * Math.sin(lngDelta / 2) ** 2;

  return 2 * EARTH_RADIUS_METERS * Math.asin(Math.min(1, Math.sqrt(haversine)));
}

/**
 * Returns the clicked pin followed by its nearest loaded neighbors within a
 * radius. The map uses this loaded viewport data to frame a useful cluster of
 * pins without issuing another request.
 */
export function selectNearbyPins(
  clickedId: string,
  pins: readonly LocatedPin[],
  radiusMeters = PIN_NEIGHBOR_RADIUS_METERS,
  limit = PIN_NEIGHBOR_LIMIT
): PinCoordinate[] {
  const clickedPin = pins.find((pin) => pin.id === clickedId);
  if (!clickedPin) return [];

  const clickedPoint = parsePoint(clickedPin.location);
  if (!clickedPoint) return [];

  const clicked: PinCoordinate = { id: clickedId, ...clickedPoint };
  if (!Number.isFinite(radiusMeters) || radiusMeters < 0 || limit <= 0) {
    return [clicked];
  }

  const nearby: Array<{ point: PinCoordinate; distance: number }> = [];
  for (const pin of pins) {
    if (pin.id === clickedId) continue;
    const point = parsePoint(pin.location);
    if (!point) continue;

    const distance = distanceMeters(clickedPoint, point);
    if (distance <= radiusMeters) {
      nearby.push({ point: { id: pin.id, ...point }, distance });
    }
  }

  nearby.sort((first, second) => {
    const distanceDifference = first.distance - second.distance;
    return distanceDifference || first.point.id.localeCompare(second.point.id);
  });

  return [clicked, ...nearby.slice(0, limit).map(({ point }) => point)];
}

export type GeoCoords = {
  lat: number;
  lng: number;
};

const DEFAULT_POSITION_OPTIONS: PositionOptions = {
  enableHighAccuracy: true,
  timeout: 10000,
  maximumAge: 60000,
};

export function geolocationAvailable(): boolean {
  return typeof navigator !== "undefined" && "geolocation" in navigator;
}

/**
 * Promise wrapper around navigator.geolocation.getCurrentPosition with a
 * shared default option set. Rejects with an Error (never a raw
 * GeolocationPositionError) so callers can rely on `.message`.
 */
export function getCurrentPosition(
  opts?: Partial<PositionOptions>
): Promise<GeolocationPosition> {
  return new Promise((resolve, reject) => {
    if (!geolocationAvailable()) {
      reject(new Error("Location is not available in this browser"));
      return;
    }
    navigator.geolocation.getCurrentPosition(
      resolve,
      (err) => reject(new Error(err.message || "Could not get your location")),
      { ...DEFAULT_POSITION_OPTIONS, ...opts }
    );
  });
}

export function toGeoCoords(pos: GeolocationPosition): GeoCoords {
  return { lat: pos.coords.latitude, lng: pos.coords.longitude };
}

export function boundsToValidBbox(b: LngLatBounds): [number, number, number, number] {
  const south = Math.max(-90, Math.min(90, b.getSouth()));
  const north = Math.max(-90, Math.min(90, b.getNorth()));
  const west = b.getWest();
  const east = b.getEast();
  if (east - west >= 360) return [south, -180, north, 180];
  const wrap = (lng: number) => ((((lng % 360) + 540) % 360) - 180);
  const lngMin = wrap(west);
  const lngMax = wrap(east);
  if (lngMax < lngMin) return [south, -180, north, 180];
  return [south, lngMin, north, lngMax];
}