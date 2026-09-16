import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { User } from "@/lib/types";

interface AuthState {
  user: User | null;
  token: string | null;
  setAuth: (user: User, token: string) => void;
  clearAuth: () => void;
}

function pruneUser(user: User | null) {
  if (!user) return null;
  return {
    id: user.id,
    username: user.username,
    avatar_url: user.avatar_url,
    role: user.role,
  };
}

export const useAuth = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      setAuth: (user, token) => set({ user, token }),
      clearAuth: () => set({ user: null, token: null }),
    }),
    {
      name: "goodspot-auth",
      partialize: (state) => ({
        token: state.token,
        user: pruneUser(state.user),
      }),
    }
  )
);