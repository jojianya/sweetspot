import api, { bearer } from "./client";
import type { AuthResponse } from "@/lib/types";

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