import api from "./client";
import type { AuthResponse } from "@/lib/types";

export async function login(identifier: string, password: string): Promise<AuthResponse> {
  const { data } = await api.post<AuthResponse>("/auth/login", {
    identifier,
    password,
  });
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

export async function logout(): Promise<void> {
  await api.post("/auth/logout");
}