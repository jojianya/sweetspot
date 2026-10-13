"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import ThemeToggle from "./ThemeToggle";
import Avatar from "@/components/Avatar";
import { BookmarkIcon, PersonIcon } from "@/components/icons";
import { useLogout } from "@/hooks/useLogout";
import { useSessionRefresh } from "@/hooks/useSessionRefresh";
import { useAuth } from "@/store/auth";

/** Account cluster in the map nav bar: avatar menu when signed in, auth CTAs otherwise. */
export function AccountMenu({ onOpenSaved }: { onOpenSaved: () => void }) {
  const { user } = useAuth();
  const router = useRouter();
  const isLoggedIn = user !== null;
  const handleLogout = useLogout();

  // Keep the cached role in step with the DB so moderation menu items appear
  // for freshly promoted accounts without re-logging in.
  useSessionRefresh();

  const [profileOpen, setProfileOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close the account menu on outside click or Escape; Escape returns focus
  // to the trigger so keyboard users land where they started.
  useEffect(() => {
    if (!profileOpen) return;
    const onPointerDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (menuRef.current?.contains(target) || triggerRef.current?.contains(target)) return;
      setProfileOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setProfileOpen(false);
        triggerRef.current?.focus();
      }
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [profileOpen]);

  // Arrow-key navigation between menu items (roving focus, wraps around).
  const handleMenuKeyDown = (e: React.KeyboardEvent) => {
    const items = Array.from(
      menuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []
    );
    if (items.length === 0) return;
    const current = items.indexOf(document.activeElement as HTMLElement);
    let next = -1;
    if (e.key === "ArrowDown") next = current < 0 ? 0 : (current + 1) % items.length;
    else if (e.key === "ArrowUp") next = current < 0 ? items.length - 1 : (current - 1 + items.length) % items.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = items.length - 1;
    else return;
    e.preventDefault();
    items[next].focus();
  };

  const menuButtonClass =
    "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800";

  return (
    <div className="order-2 flex shrink-0 items-center gap-2.5 sm:order-none">
      {isLoggedIn && user ? (
        <div className="relative">
          <button
            type="button"
            ref={triggerRef}
            onClick={() => setProfileOpen((o) => !o)}
            aria-label="Account"
            aria-haspopup="menu"
            aria-expanded={profileOpen}
            className="block h-10 w-10 overflow-hidden rounded-full border border-[#E0E0E0] transition-transform hover:scale-105 dark:border-zinc-700"
          >
            <Avatar
              src={user.avatar_url}
              username={user.username}
              alt={user.username}
              className="h-full w-full"
            />
          </button>
          {profileOpen && (
            <div
              ref={menuRef}
              onKeyDown={handleMenuKeyDown}
              className="absolute right-0 top-full z-30 mt-2 w-56 overflow-hidden rounded-2xl border border-zinc-200/70 bg-white shadow-xl shadow-zinc-900/10 dark:border-zinc-700/70 dark:bg-zinc-900"
              role="menu"
            >
              <div className="flex items-center gap-3 border-b border-zinc-100 px-4 py-3 dark:border-zinc-800">
                <Avatar
                  src={user.avatar_url}
                  username={user.username}
                  className="h-9 w-9"
                />
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">@{user.username}</p>
                  <p className="truncate text-xs capitalize text-zinc-500 dark:text-zinc-400">{user.role}</p>
                </div>
              </div>
              <div className="p-2">
                <ThemeToggle />
                <button
                  type="button"
                  onClick={() => {
                    setProfileOpen(false);
                    onOpenSaved();
                  }}
                  role="menuitem"
                  className={menuButtonClass}
                >
                  <BookmarkIcon className="h-4 w-4 text-zinc-400" />
                  Saved
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setProfileOpen(false);
                    router.push(`/users/${user.id}`);
                  }}
                  role="menuitem"
                  className={menuButtonClass}
                >
                  <PersonIcon className="h-4 w-4 text-zinc-400" />
                  My profile
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setProfileOpen(false);
                    router.push("/feed");
                  }}
                  role="menuitem"
                  className={menuButtonClass}
                >
                  <svg className="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                    <path d="M4 19V5a2 2 0 0 1 2-2h13v16" />
                    <path d="M4 19a2 2 0 0 0 2 2h13" />
                    <path d="M9 7h6M9 11h6M9 15h4" />
                  </svg>
                  Feed
                </button>
                {(user.role === "admin" || user.role === "owner") && (
                  <button
                    type="button"
                    onClick={() => {
                      setProfileOpen(false);
                      router.push("/reports");
                    }}
                    role="menuitem"
                    className={menuButtonClass}
                  >
                    <svg className="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                      <path d="M12 2 3 7v6c0 5 4 8 9 9 5-1 9-4 9-9V7l-9-5Z" />
                      <path d="M12 8v4M12 16h.01" />
                    </svg>
                    Reports
                  </button>
                )}
                {user.role === "owner" && (
                  <button
                    type="button"
                    onClick={() => {
                      setProfileOpen(false);
                      router.push("/roles");
                    }}
                    role="menuitem"
                    className={menuButtonClass}
                  >
                    <svg className="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                      <circle cx="9" cy="8" r="3.5" />
                      <path d="M2.5 20c0-3.6 2.9-6 6.5-6s6.5 2.4 6.5 6" />
                      <path d="M16 4.5a3.5 3.5 0 0 1 0 7M21.5 20c0-2.6-1.7-4.6-4-5.3" />
                    </svg>
                    Roles
                  </button>
                )}
                <button
                  type="button"
                  onClick={handleLogout}
                  role="menuitem"
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm text-rose-600 hover:bg-rose-50 dark:text-rose-400 dark:hover:bg-rose-950/40"
                >
                  <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                    <path d="m16 17 5-5-5-5" />
                    <path d="M21 12H9" />
                  </svg>
                  Log out
                </button>
              </div>
            </div>
          )}
        </div>
      ) : (
        <>
          <button
            type="button"
            onClick={() => router.push("/login")}
            className="rounded-full border border-[#E0E0E0] bg-white px-4 py-2 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Log in
          </button>
          <button
            type="button"
            onClick={() => router.push("/register")}
            className="rounded-full bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2 text-sm font-medium text-white transition-transform hover:brightness-105 active:scale-95"
          >
            Join
          </button>
        </>
      )}
    </div>
  );
}
