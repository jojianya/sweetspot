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
        <Link
          href="/"
          className="pointer-events-auto flex shrink-0 items-center gap-2.5 [text-shadow:0_1px_2px_rgba(255,255,255,0.9)]"
        >
          <span className="flex h-9 w-9 items-center justify-center rounded-full bg-gradient-to-br from-rose-500 to-rose-700 text-white shadow-md shadow-rose-600/30">
            <svg className="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2a7 7 0 0 0-7 7c0 5.25 7 13 7 13s7-7.75 7-13a7 7 0 0 0-7-7Zm0 9.5A2.5 2.5 0 1 1 12 6.5a2.5 2.5 0 0 1 0 5Z" />
            </svg>
          </span>
          <span
            className={`text-lg font-semibold tracking-tight text-zinc-900 ${
              children ? "hidden sm:inline" : "inline"
            }`}
          >
            Goodspot
          </span>
        </Link>

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