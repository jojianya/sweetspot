"use client";

import { useRef, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import SearchBar from "./SearchBar";
import ThemeToggle from "./ThemeToggle";
import { logout } from "@/lib/api";
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

type IconName =
  | "restaurant"
  | "hotel"
  | "activities"
  | "museum"
  | "transit"
  | "pharmacy"
  | "park"
  | "shopping"
  | "bar"
  | "religion"
  | "tag";

function categoryIcon(name: string): IconName {
  const n = name.toLowerCase();
  if (/restaurant|food|cafe|dining|eat|coffee|brunch/.test(n)) return "restaurant";
  if (/hotel|stay|lodge|inn|accommodation|motel|resort|hostel|apartment/.test(n)) return "hotel";
  if (/thing|attraction|tour|activity|see|fun|outdoor|adventure/.test(n)) return "activities";
  if (/museum|gallery|art|history|exhibit|heritage/.test(n)) return "museum";
  if (/transit|transport|train|bus|metro|rail|station|airport/.test(n)) return "transit";
  if (/pharm|drug|medic|chemist|clinic|health|wellness/.test(n)) return "pharmacy";
  if (/park|garden|nature|trail|playground|reserve/.test(n)) return "park";
  if (/shop|store|mall|market|retail|boutique/.test(n)) return "shopping";
  if (/bar|pub|night|club|cocktail|drink|brew/.test(n)) return "bar";
  if (/temple|church|mosque|synagogue|worship|holy|shrine/.test(n)) return "religion";
  return "tag";
}

function ChipIcon({ name }: { name: string }) {
  const icon = categoryIcon(name);
  const common = {
    width: 14,
    height: 14,
    viewBox: "0 0 24 24",
    fill: "none" as const,
    stroke: "currentColor",
    strokeWidth: 2,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };
  if (name.trim().toLowerCase() === "all") {
    return (
      <svg {...common}>
        <circle cx="12" cy="12" r="10" />
        <path d="M12 2a14 14 0 0 0 0 20 14 14 0 0 0 0-20" />
        <path d="M2 12h20" />
      </svg>
    );
  }
  switch (icon) {
    case "restaurant":
      return (
        <svg {...common}>
          <path d="M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2" />
          <path d="M7 2v20" />
          <path d="M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3Zm0 0v7" />
        </svg>
      );
    case "hotel":
      return (
        <svg {...common}>
          <path d="M2 20v-8a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v8" />
          <path d="M4 10V6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v4" />
          <path d="M12 4v6" />
          <path d="M2 18h20" />
        </svg>
      );
    case "activities":
      return (
        <svg {...common}>
          <path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z" />
          <circle cx="12" cy="13" r="3" />
        </svg>
      );
    case "museum":
      return (
        <svg {...common}>
          <line x1="3" x2="21" y1="22" y2="22" />
          <line x1="6" x2="6" y1="18" y2="11" />
          <line x1="10" x2="10" y1="18" y2="11" />
          <line x1="14" x2="14" y1="18" y2="11" />
          <line x1="18" x2="18" y1="18" y2="11" />
          <polygon points="12 2 20 7 4 7" />
        </svg>
      );
    case "transit":
      return (
        <svg {...common}>
          <path d="M8 3.1V7a4 4 0 0 0 8 0V3.1" />
          <path d="m9 15-1-1" />
          <path d="m15 15 1-1" />
          <path d="M9 19c-2.8 0-5-2.2-5-5v-4a8 8 0 0 1 16 0v4c0 2.8-2.2 5-5 5Z" />
          <path d="m8 19-2 3" />
          <path d="m16 19 2 3" />
        </svg>
      );
    case "pharmacy":
      return (
        <svg {...common}>
          <path d="m10.5 20.5 10-10a4.95 4.95 0 1 0-7-7l-10 10a4.95 4.95 0 1 0 7 7Z" />
          <path d="m8.5 8.5 7 7" />
        </svg>
      );
    case "park":
      return (
        <svg {...common}>
          <path d="m17 14 3 3.3a1 1 0 0 1-.7 1.7H4.7a1 1 0 0 1-.7-1.7L7 14h-.3a1 1 0 0 1-.7-1.7L9 9h-.2A1 1 0 0 1 8 7.3L12 3l4 4.3a1 1 0 0 1-.8 1.7H15l3 3.3a1 1 0 0 1-.7 1.7H17Z" />
          <path d="M12 22v-3" />
        </svg>
      );
    case "shopping":
      return (
        <svg {...common}>
          <path d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4Z" />
          <path d="M3 6h18" />
          <path d="M16 10a4 4 0 0 1-8 0" />
        </svg>
      );
    case "bar":
      return (
        <svg {...common}>
          <path d="M8 22h8" />
          <path d="M12 11v11" />
          <path d="m19 3-7 8-7-8Z" />
        </svg>
      );
    case "religion":
      return (
        <svg {...common}>
          <path d="M10 9h4" />
          <path d="M12 7v5" />
          <path d="M14 22v-4a2 2 0 0 0-4 0v4" />
          <path d="M18 22V5.618a1 1 0 0 0-.553-.894l-4.553-2.277a2 2 0 0 0-1.788 0L6.553 4.724A1 1 0 0 0 6 5.618V22" />
          <path d="m18 7 3.447 1.724a1 1 0 0 1 .553.894V20a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9.618a1 1 0 0 1 .553-.894L6 7" />
        </svg>
      );
    default:
      return (
        <svg {...common}>
          <path d="M12.586 2.586A2 2 0 0 0 11.172 2H4a2 2 0 0 0-2 2v7.172a2 2 0 0 0 .586 1.414l8.704 8.704a2.426 2.426 0 0 0 3.42 0l6.58-6.58a2.426 2.426 0 0 0 0-3.42z" />
          <circle cx="7.5" cy="7.5" r="0.5" fill="currentColor" stroke="none" />
        </svg>
      );
  }
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
  const { user, token, clearAuth } = useAuth();
  const router = useRouter();
  const isLoggedIn = token !== null;

  const scrollRef = useRef<HTMLDivElement>(null);
  const [profileOpen, setProfileOpen] = useState(false);
  const [appsOpen, setAppsOpen] = useState(false);

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // token may already be blacklisted; clear locally regardless
    }
    clearAuth();
    router.push("/");
  };

  const scrollChips = (dir: 1 | -1) => {
    scrollRef.current?.scrollBy({ left: dir * 240, behavior: "smooth" });
  };

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
            {user.avatar_url ? (
              // eslint-disable-next-line @next/next/no-img-element -- avatars are hosted media, <img> is fine here
              <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
            ) : (
              <span className="flex h-full w-full items-center justify-center bg-zinc-100 text-sm font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                {user.username.charAt(0).toUpperCase()}
              </span>
            )}
          </button>
          {profileOpen && (
            <div
              className="absolute right-0 top-full z-30 mt-2 w-56 overflow-hidden rounded-2xl border border-zinc-200/70 bg-white shadow-xl shadow-zinc-900/10 dark:border-zinc-700/70 dark:bg-zinc-900"
              role="menu"
            >
              <div className="flex items-center gap-3 border-b border-zinc-100 px-4 py-3 dark:border-zinc-800">
                {user.avatar_url ? (
                  // eslint-disable-next-line @next/next/no-img-element -- avatars are hosted media, <img> is fine here
                  <img
                    src={user.avatar_url}
                    alt=""
                    className="h-9 w-9 rounded-full object-cover"
                  />
                ) : (
                  <span className="flex h-9 w-9 items-center justify-center rounded-full bg-zinc-100 text-sm font-semibold text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                    {user.username.charAt(0).toUpperCase()}
                  </span>
                )}
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
                  className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800"
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