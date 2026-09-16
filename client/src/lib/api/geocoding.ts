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
    types: "place,address,street,locality,neighborhood,region,country",
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
