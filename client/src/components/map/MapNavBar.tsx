"use client";

import { useRef, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import SearchBar from "./SearchBar";
import ThemeToggle from "./ThemeToggle";
import { ChipIcon } from "./CategoryIcons";
import Avatar from "@/components/Avatar";
import { useLogout } from "@/hooks/useLogout";
import { useAuth } from "@/store/auth";
import type { Category, PinListEntry } from "@/lib/types";

interface MapNavBarProps {
  center: { lat: number; lng: number };
  onSelectPlace: (center: { lat: number; lng: number }, bbox?: [number, number, number, number]) => void;
  onSelectPin: (entry: PinListEntry) => void;
  categories: Category[];
  selectedCategory: number | null;
  onSelectCategory: (id: number | null) => void;
  onOpenSaved: () => void;
}

function AppsIcon() {
  return (
    <svg className="h-[18px] w-[18px]" viewBox="0 0 24 24" aria-hidden>
      {[
        [5, 5],
        [12, 5],
        [19, 5],
        [5, 12],
        [12, 12],
        [19, 12],
        [5, 19],
        [12, 19],
        [19, 19],
      ].map(([cx, cy]) => (
        <circle key={`${cx}-${cy}`} cx={cx} cy={cy} r="1.6" fill="currentColor" />
      ))}
    </svg>
  );
}

function ChevronRightIcon() {
  return (
    <svg
      className="h-[18px] w-[18px]"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}

export default function MapNavBar({
  center,
  onSelectPlace,
  onSelectPin,
  categories,
  selectedCategory,
  onSelectCategory,
  onOpenSaved,
}: MapNavBarProps) {
  const { user, token } = useAuth();
  const router = useRouter();
  const isLoggedIn = token !== null;
  const handleLogout = useLogout();

  const scrollRef = useRef<HTMLDivElement>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [appsOpen, setAppsOpen] = useState(false);

  const scrollChips = (dir: 1 | -1) => {
    scrollRef.current?.scrollBy({ left: dir * 240, behavior: "smooth" });
  };

  const menuButtonClass =
    "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800";

  const rightCluster: ReactNode = (
    <div className="flex shrink-0 items-center gap-2.5">
      <div className="relative">
        <button
          type="button"
          onClick={() => {
            setAppsOpen((o) => !o);
            setProfileOpen(false);
          }}
          aria-label="Apps"
          aria-expanded={appsOpen}
          className="flex h-10 w-10 items-center justify-center rounded-full border border-[#E0E0E0] bg-white text-zinc-500 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800"
        >
          <AppsIcon />
        </button>
        {appsOpen && (
          <div
            className="absolute right-0 top-full z-30 mt-2 w-56 overflow-hidden rounded-2xl border border-zinc-200/70 bg-white shadow-xl shadow-zinc-900/10 dark:border-zinc-700/70 dark:bg-zinc-900"
            role="menu"
          >
            <div className="flex flex-col p-2">
              <span className="px-3 py-2 text-sm text-zinc-500 dark:text-zinc-400">
                {categories.length} categories on the map
              </span>
              <div className="my-1 h-px bg-zinc-100 dark:bg-zinc-800" />
              <button
                type="button"
                onClick={() => {
                  setAppsOpen(false);
                  onSelectCategory(null);
                }}
                role="menuitem"
                className="flex items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                <svg className="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
                  <path d="M3 21V9l9-6 9 6v12" />
                  <path d="M9 21v-6h6v6" />
                </svg>
                Show all pins
              </button>
            </div>
          </div>
        )}
      </div>

      {isLoggedIn && user ? (
        <div className="relative">
          <button
            type="button"
            onClick={() => {
              setProfileOpen((o) => !o);
              setAppsOpen(false);
            }}
            aria-label="Account"
            aria-expanded={profileOpen}
            className="block h-10 w-10 overflow-hidden rounded-full border border-[#E0E0E0] transition-transform hover:scale-105 dark:border-zinc-700"
          >
            <Avatar
              src={user.avatar_url}
              username={user.username}
              className="h-full w-full"
            />
          </button>
          {profileOpen && (
            <div
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
                    setAppsOpen(false);
                    onOpenSaved();
                  }}
                  role="menuitem"
                  className={menuButtonClass}
                >
                  <svg className="h-4 w-4 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                    <path d="M19 21 12 16 5 21V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2Z" />
                  </svg>
                  Saved
                </button>
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

  return (
    <header className="pointer-events-none absolute inset-x-0 top-0 z-30">
      <div className="pointer-events-auto flex items-center gap-2.5 px-2.5 py-2 sm:gap-3 sm:px-3">
          <div className="w-full min-w-0 flex-1 sm:w-72 sm:flex-none sm:min-w-64">
            <SearchBar
              center={center}
              onSelectPlace={onSelectPlace}
              onSelectPin={onSelectPin}
              placeholder="Search Maps"
              className="w-full min-w-0"
            />
          </div>

          <div className="flex min-w-0 flex-1 items-center gap-2">
            <div
              ref={scrollRef}
              className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
            >
              {(() => {
                const active = selectedCategory === null;
                return (
                  <button
                    type="button"
                    onClick={() => onSelectCategory(null)}
                    aria-pressed={active}
                    className={
                      "flex h-9 shrink-0 items-center gap-1.5 rounded-full border px-2.5 text-[13px] font-medium transition-colors " +
                      (active
                        ? "border-blue-500 bg-blue-50 text-blue-600 dark:border-blue-500 dark:bg-blue-950/60 dark:text-blue-400"
                        : "border-[#E0E0E0] bg-white text-zinc-800 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:bg-zinc-800")
                    }
                  >
                    <ChipIcon name="All" />
                    <span className="whitespace-nowrap">All</span>
                  </button>
                );
              })()}
              {categories.map((c) => {
                const active = selectedCategory === c.id;
                return (
                  <button
                    key={c.id}
                    type="button"
                    onClick={() => onSelectCategory(active ? null : c.id)}
                    aria-pressed={active}
                    className={
                      "flex h-9 shrink-0 items-center gap-1.5 rounded-full border px-2.5 text-[13px] font-medium transition-colors " +
                      (active
                        ? "border-blue-500 bg-blue-50 text-blue-600 dark:border-blue-500 dark:bg-blue-950/60 dark:text-blue-400"
                        : "border-[#E0E0E0] bg-white text-zinc-800 hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:bg-zinc-800")
                    }
                  >
                    <ChipIcon name={c.name} />
                    <span className="whitespace-nowrap">{c.name}</span>
                  </button>
                );
              })}
            </div>
            <button
              type="button"
              onClick={() => scrollChips(1)}
              aria-label="More categories"
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-[#E0E0E0] bg-white text-zinc-600 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800"
            >
              <ChevronRightIcon />
            </button>
          </div>

          {rightCluster}
        </div>
    </header>
  );
}