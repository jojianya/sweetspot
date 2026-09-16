"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import type { ReactNode } from "react";
import { logout } from "@/lib/api";
import { useAuth } from "@/store/auth";

interface NavbarProps {
  children?: ReactNode;
}

export default function Navbar({ children }: NavbarProps) {
  const { user, token, clearAuth } = useAuth();
  const router = useRouter();
  const isLoggedIn = token !== null;

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // token may already be blacklisted; clear locally regardless
    }
    clearAuth();
    router.push("/");
  };

  return (
    <header className="pointer-events-none absolute inset-x-0 top-0 z-30">
      <div className="flex items-center gap-3 px-4 py-3">
        {children && (
          <div className="pointer-events-auto min-w-0 flex-1 justify-center">
            {children}
          </div>
        )}

        <nav className="pointer-events-auto ml-auto flex shrink-0 items-center gap-2.5">
          {isLoggedIn && user ? (
            <>
              <div className="hidden items-center gap-2.5 sm:flex [text-shadow:0_1px_2px_rgba(255,255,255,0.9)]">
                {user.avatar_url ? (
                  <img
                    src={user.avatar_url}
                    alt=""
                    className="h-8 w-8 rounded-full object-cover"
                  />
                ) : (
                  <span className="flex h-8 w-8 items-center justify-center rounded-full bg-rose-100 text-sm font-semibold text-rose-700">
                    {user.username.charAt(0).toUpperCase()}
                  </span>
                )}
                <span className="text-sm text-zinc-600">@{user.username}</span>
              </div>
              <button
                type="button"
                onClick={handleLogout}
                className="rounded-full border border-zinc-300 bg-white/90 px-3.5 py-1.5 text-sm font-medium text-zinc-700 shadow-sm backdrop-blur transition-colors hover:bg-white"
              >
                Log out
              </button>
            </>
          ) : (
            <>
              <Link
                href="/login"
                className="rounded-full border border-zinc-300 bg-white/90 px-3.5 py-1.5 text-sm font-medium text-zinc-700 shadow-sm backdrop-blur transition-colors hover:bg-white"
              >
                Log in
              </Link>
              <Link
                href="/register"
                className="rounded-full bg-gradient-to-r from-rose-600 to-rose-500 px-3.5 py-1.5 text-sm font-medium text-white shadow-md shadow-rose-600/25 transition-transform hover:shadow-lg hover:shadow-rose-600/30 active:scale-95"
              >
                Join
              </Link>
            </>
          )}
        </nav>
      </div>
    </header>
  );
}