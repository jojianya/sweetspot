import api, { bearer } from "./client";
import type { CreatedPin, NewPinPhoto, PinDetail, PinListEntry } from "@/lib/types";

export async function fetchPins(
  bbox: string,
  category: number | null,
  token: string | null
): Promise<PinListEntry[]> {
  const params: Record<string, string> = { bbox };
  if (category !== null) params.category = String(category);

  const { data } = await api.get<{ pins: PinListEntry[] }>("/pins", {
    params,
    headers: bearer(token),
  });
  return data.pins;
}

export async function fetchPin(id: string, token: string | null): Promise<PinDetail> {
  const { data } = await api.get<{ pin: PinDetail }>(`/pins/${id}`, {
    headers: bearer(token),
  });
  return data.pin;
}

export async function createPin(
  token: string,
  form: {
    lat: number;
    lng: number;
    categoryId: number;
    caption: string | null;
    photos: File[];
  }
): Promise<{ pin: CreatedPin; photos: NewPinPhoto[] }> {
  const fd = new FormData();
  fd.append("lat", String(form.lat));
  fd.append("lng", String(form.lng));
  fd.append("category_id", String(form.categoryId));
  if (form.caption) fd.append("caption", form.caption);
  for (const file of form.photos) fd.append("photos", file);

  const { data } = await api.post<{ pin: CreatedPin; photos: NewPinPhoto[] }>(
    "/pins",
    fd,
    { headers: bearer(token) }
  );
  return data;
}