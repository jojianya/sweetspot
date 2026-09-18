"use client";

import { useCallback } from "react";
import { useRouter } from "next/navigation";
import { logout } from "@/lib/api";
import { useAuth } from "@/store/auth";

export function useLogout() {
  const clearAuth = useAuth((s) => s.clearAuth);
  const router = useRouter();

  return useCallback(async () => {
    try {
      await logout();
    } catch {
      // token may already be blacklisted; clear locally regardless
    }
    clearAuth();
    router.push("/");
  }, [clearAuth, router]);
}
