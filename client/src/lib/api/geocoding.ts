const MAPTILER_KEY = process.env.NEXT_PUBLIC_MAPTILER_API_KEY!;

export interface PlaceResult {
  id: string;
  text: string;
  place_name: string;
  center: { lat: number; lng: number };
  bbox?: [number, number, number, number];
}

interface Feature {
  id: string;
  text: string;
  place_name: string;
  geometry: { coordinates: [number, number] };
  bbox?: [number, number, number, number];
}

export async function searchPlaces(
  query: string,
  proximity?: { lat: number; lng: number },
  signal?: AbortSignal
): Promise<PlaceResult[]> {
  const params = new URLSearchParams({
    key: MAPTILER_KEY,
    language: "en",
    limit: "6",
    types: "place,municipality,municipal_district,locality,neighbourhood,address,road,county,region,country",
  });
  if (proximity) params.set("proximity", `${proximity.lng},${proximity.lat}`);

  const res = await fetch(
    `https://api.maptiler.com/geocoding/${encodeURIComponent(query)}.json?${params}`,
    { signal }
  );
  if (!res.ok) throw new Error(`Geocoding failed (${res.status})`);

  const data = (await res.json()) as { features: Feature[] };
  return data.features.map((f) => ({
    id: f.id,
    text: f.text,
    place_name: f.place_name,
    center: { lat: f.geometry.coordinates[1], lng: f.geometry.coordinates[0] },
    bbox: f.bbox,
  }));
}

/**
 * Reverse geocode a coordinate into a human-readable address string
 * (e.g. "MG Road, Indiranagar, Bengaluru"). Returns null when the lookup
 * fails or yields nothing usable.
 */
export async function reverseGeocode(
  coord: { lat: number; lng: number },
  signal?: AbortSignal
): Promise<string | null> {
  const params = new URLSearchParams({
    key: MAPTILER_KEY,
    language: "en",
    limit: "1",
    types: "address,road,neighbourhood,locality,municipality,place,region,country",
  });

  const res = await fetch(
    `https://api.maptiler.com/geocoding/${coord.lng},${coord.lat}.json?${params}`,
    { signal }
  );
  if (!res.ok) return null;

  const data = (await res.json()) as { features?: Feature[] };
  const text = data.features?.[0]?.place_name;
  return text || null;
}
