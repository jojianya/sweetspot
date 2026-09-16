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