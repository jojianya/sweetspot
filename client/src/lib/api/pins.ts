import api from "./client";
import {
  createdPinSchema,
  newPinPhotoSchema,
  pinDetailSchema,
  pinListEntrySchema,
} from "./schemas";
import type { CreatedPin, NewPinPhoto, PinDetail, PinListEntry } from "@/lib/types";

export async function fetchPins(
  bbox: string,
  category: number | null,
  signal?: AbortSignal
): Promise<PinListEntry[]> {
  const params: Record<string, string> = { bbox };
  if (category !== null) params.category = String(category);

  const { data } = await api.get<{ pins: unknown }>("/pins", { params, signal });
  return pinListEntrySchema.array().parse(data.pins);
}

export async function fetchPin(id: string, signal?: AbortSignal): Promise<PinDetail> {
  const { data } = await api.get<{ pin: unknown }>(`/pins/${id}`, { signal });
  return pinDetailSchema.parse(data.pin);
}

export async function createPin(form: {
  lat: number;
  lng: number;
  categoryId: number;
  caption: string | null;
  photos: File[];
}): Promise<{ pin: CreatedPin; photos: NewPinPhoto[] }> {
  const fd = new FormData();
  fd.append("lat", String(form.lat));
  fd.append("lng", String(form.lng));
  fd.append("category_id", String(form.categoryId));
  if (form.caption) fd.append("caption", form.caption);
  for (const file of form.photos) fd.append("photos", file);

  const { data } = await api.post<{ pin: unknown; photos: unknown }>("/pins", fd);
  return {
    pin: createdPinSchema.parse(data.pin),
    photos: newPinPhotoSchema.array().parse(data.photos),
  };
}

export async function searchPins(
  query: string,
  limit = 10,
  signal?: AbortSignal
): Promise<PinListEntry[]> {
  const { data } = await api.get<{ pins: unknown }>("/pins/search", {
    params: { q: query, limit },
    signal,
  });
  return pinListEntrySchema.array().parse(data.pins);
}