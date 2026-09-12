import type { LngLatBounds } from "maplibre-gl";

export function parsePoint(point: string): { lng: number; lat: number } {
  const m = point.match(/POINT\s*\(\s*(-?[\d.]+)\s+(-?[\d.]+)/i);
  if (!m) return { lng: 0, lat: 0 };
  return { lng: parseFloat(m[1]), lat: parseFloat(m[2]) };
}

export function formatBboxCenter(latlng: string[]): { lat: number; lng: number } {
  const [minLat, minLng, maxLat, maxLng] = latlng.map(Number);
  return { lat: (minLat + maxLat) / 2, lng: (minLng + maxLng) / 2 };
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