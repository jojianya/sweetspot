import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { User } from "@/lib/types";

export type { User };

// PersistedUser is the subset of User that is safe to store in localStorage.
// Email, socials, and timestamps are deliberately omitted — they are either
// sensitive or can be re-fetched. The session token is NOT here; it lives in
// an httpOnly cookie.
export type PersistedUser = Pick<User, "id" | "username" | "avatar_url" | "role">;

interface AuthState {
  user: PersistedUser | null;
  setUser: (user: User | null) => void;
  clearAuth: () => void;
}

function pruneUser(user: PersistedUser | null): PersistedUser | null {
  if (!user) return null;
  return {
    id: user.id,
    username: user.username,
    avatar_url: user.avatar_url,
    role: user.role,
  };
}

// The session token lives in an httpOnly cookie set by the server, so it is
// never readable from JavaScript and never touches localStorage. Only the
// non-sensitive user object is persisted here, and only to keep the UI in
// step across reloads — the cookie is the actual credential and is sent
// automatically by the browser on same-origin requests.
export const useAuth = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      setUser: (user) => set({ user: pruneUser(user) }),
      clearAuth: () => set({ user: null }),
    }),
    {
      name: "goodspot-auth",
      partialize: (state) => ({
        user: pruneUser(state.user) as PersistedUser | null,
      }),
    }
  )
);
