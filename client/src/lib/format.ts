export function parsePoint(point: string): { lng: number; lat: number } {
  const m = point.match(/POINT\s*\(\s*(-?[\d.]+)\s+(-?[\d.]+)/i);
  if (!m) return { lng: 0, lat: 0 };
  return { lng: parseFloat(m[1]), lat: parseFloat(m[2]) };
}

export function formatBboxCenter(latlng: string[]): { lat: number; lng: number } {
  const [minLat, minLng, maxLat, maxLng] = latlng.map(Number);
  return { lat: (minLat + maxLat) / 2, lng: (minLng + maxLng) / 2 };
}

export function formatTime(iso: string): string {
  return new Date(iso).toLocaleString();
}