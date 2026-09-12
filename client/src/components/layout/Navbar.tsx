"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { logout } from "@/lib/api";
import { useAuth } from "@/store/auth";

export default function Navbar() {
  const { user, token, isLoggedIn, clearAuth } = useAuth();
  const router = useRouter();

  const handleLogout = async () => {
    try {
      await logout(token);
    } catch {
      // token may already be blacklisted; clear locally regardless
    }
    clearAuth();
    router.push("/");
  };

  return (
    <header className="flex items-center justify-between border-b border-zinc-200 bg-white px-4 py-3">
      <Link href="/" className="flex items-center gap-2">
        <span className="flex h-8 w-8 items-center justify-center rounded-full bg-rose-600 text-lg text-white">
          ⁂
        </span>
        <span className="text-lg font-semibold text-zinc-900">Goodspot</span>
      </Link>

      <nav className="flex items-center gap-3">
        {isLoggedIn && user ? (
          <>
            <span className="hidden text-sm text-zinc-600 sm:inline">
              @{user.username}
            </span>
            <button
              type="button"
              onClick={handleLogout}
              className="rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium text-zinc-700 hover:bg-zinc-50"
            >
              Log out
            </button>
          </>
        ) : (
          <>
            <Link
              href="/login"
              className="rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium text-zinc-700 hover:bg-zinc-50"
            >
              Log in
            </Link>
            <Link
              href="/register"
              className="rounded-lg bg-rose-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-rose-700"
            >
              Join
            </Link>
          </>
        )}
      </nav>
    </header>
  );
}
