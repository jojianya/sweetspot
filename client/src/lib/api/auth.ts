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

export async function requestPasswordReset(email: string): Promise<{ message: string }> {
  const { data } = await api.post<{ message: string }>("/auth/password/request", {
    email,
  });
  return data;
}

export async function confirmPasswordReset(token: string, password: string): Promise<{ message: string }> {
  const { data } = await api.post<{ message: string }>("/auth/password/reset", {
    token,
    password,
  });
  return data;
}

/** The authoritative answer to "is this browser still signed in?". */
export interface Session {
  user_id: string;
  role: string;
}

/**
 * Asks the server whether the session cookie is still valid.
 *
 * This must be an endpoint behind AuthRequired. GET /users/:id is not: it sits
 * behind OptionalAuth and answers 200 with the public profile even when no
 * cookie is sent, so it can never detect an ended session. /me answers 401
 * when the cookie is missing, expired, or revoked, which is the signal the
 * client needs to reconcile its cached user with reality.
 */
export async function fetchSession(): Promise<Session> {
  const { data } = await api.get<Session>("/me");
  return data;
}