export interface User {
  id: string;
  email: string;
  username: string;
  avatar_url: string | null;
  socials: Record<string, unknown>;
  role: string;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  token: string;
}