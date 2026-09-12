import axios from "axios";
import type {
  AuthResponse,
  Category,
  NewPinPhoto,
  PinDetail,
  PinListEntry,
} from "@/lib/types";

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8081";

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 20000,
});

api.interceptors.response.use(
  (res) => res,
  (error) => {
    const message: string =
      error?.response?.data?.error ?? error?.message ?? "something went wrong";
    return Promise.reject(new Error(message));
  }
);

function bearer(token: string | null) {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function fetchCategories(): Promise<Category[]> {
  const { data } = await api.get<Category[]>("/categories");
  return data;
}

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

export async function login(email: string, password: string): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/login", { email, password });
  return data;
}

export async function register(
  email: string,
  username: string,
  password: string
): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/register", {
    email,
    username,
    password,
  });
  return data;
}

export async function logout(token: string | null): Promise<void> {
  await api.post("/auth/logout", null, { headers: bearer(token) });
}

export interface CreatedPin {
  id: string;
  user_id: string;
  location: string;
  geohash: string;
  caption: string | null;
  category_id: number;
  is_hidden: boolean;
  created_at: string;
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

  const { data } = await api.post<{
    pin: CreatedPin;
    photos: NewPinPhoto[];
  }>("/pins", fd, { headers: bearer(token) });
  return data;
}