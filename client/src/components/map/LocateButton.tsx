"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const GEO_AVAILABLE = typeof navigator !== "undefined" && "geolocation" in navigator;

interface Props {
  onLocate: (pos: { lat: number; lng: number }) => void;
}

export default function LocateButton({ onLocate }: Props) {
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const timerRef = useRef<ReturnType<typeof setTimeout>>(null);

  const handleClick = useCallback(() => {
    if (status === "loading") return;
    setStatus("loading");
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        onLocate({ lat: pos.coords.latitude, lng: pos.coords.longitude });
        setStatus("idle");
      },
      () => {
        setStatus("error");
        clearTimeout(timerRef.current!);
        timerRef.current = setTimeout(() => setStatus("idle"), 2000);
      },
      { enableHighAccuracy: true, timeout: 10000, maximumAge: 0 }
    );
  }, [status, onLocate]);

  useEffect(() => () => clearTimeout(timerRef.current!), []);

  if (!GEO_AVAILABLE) return null;

  return (
    <button
      type="button"
      onClick={handleClick}
      title="Go to my location"
      className={`flex h-10 w-10 items-center justify-center rounded-full bg-white shadow-md ring-1 ring-zinc-200 transition-colors hover:bg-zinc-50 dark:bg-zinc-900 dark:ring-zinc-700 dark:hover:bg-zinc-800 ${status === "error" ? "ring-rose-400" : ""}`}
    >
      {status === "loading" ? (
        <span className="block h-4 w-4 animate-spin rounded-full border-2 border-zinc-300 border-t-zinc-700 dark:border-zinc-700 dark:border-t-zinc-300" />
      ) : (
        <svg className={`h-4 w-4 ${status === "error" ? "text-rose-500" : "text-zinc-600 dark:text-zinc-400"}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="3" />
          <path d="M12 2v3M12 19v3M2 12h3M19 12h3" />
        </svg>
      )}
    </button>
  );
}
