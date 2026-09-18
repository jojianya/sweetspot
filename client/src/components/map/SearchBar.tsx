"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { searchPlaces, type PlaceResult } from "@/lib/api/geocoding";
import { searchPins } from "@/lib/api/pins";
import type { PinListEntry } from "@/lib/types";

interface ResultItem {
  kind: "place" | "pin";
  place?: PlaceResult;
  pin?: PinListEntry;
}

interface SearchBarProps {
  center: { lat: number; lng: number };
  onSelectPlace: (center: { lat: number; lng: number }, bbox?: [number, number, number, number]) => void;
  onSelectPin: (entry: PinListEntry) => void;
  placeholder?: string;
  className?: string;
  endSlot?: ReactNode;
}

export default function SearchBar({
  center,
  onSelectPlace,
  onSelectPin,
  placeholder = "Search places or pins…",
  className,
  endSlot,
}: SearchBarProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<ResultItem[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [loading, setLoading] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const centerRef = useRef(center);
  useEffect(() => {
    centerRef.current = center;
  });
  const selectedRef = useRef(false);

  const doSearch = useCallback(async (q: string, signal?: AbortSignal) => {
    setLoading(true);
    setIsOpen(true);
    try {
      const [placesRes, pinsRes] = await Promise.allSettled([
        searchPlaces(q, centerRef.current, signal),
        searchPins(q, 5, signal),
      ]);
      if (signal?.aborted) return;
      const items: ResultItem[] = [];
      if (placesRes.status === "fulfilled") {
        for (const p of placesRes.value) items.push({ kind: "place", place: p });
      }
      if (pinsRes.status === "fulfilled") {
        for (const p of pinsRes.value) items.push({ kind: "pin", pin: p });
      }
      setResults(items);
      setIsOpen(items.length > 0);
    } catch {
      if (signal?.aborted) return;
      setResults([]);
      setIsOpen(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selectedRef.current) return;
    abortRef.current?.abort();
    if (query.trim().length < 2) return;
    const controller = new AbortController();
    abortRef.current = controller;
    const timer = setTimeout(() => doSearch(query.trim(), controller.signal), 300);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [query, doSearch]);

  useEffect(() => {
    const handle = (e: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handle);
    return () => document.removeEventListener("mousedown", handle);
  }, []);

  const select = (item: ResultItem) => {
    selectedRef.current = true;
    if (item.kind === "place" && item.place) {
      onSelectPlace(item.place.center, item.place.bbox);
      setQuery(item.place.text);
    } else if (item.kind === "pin" && item.pin) {
      onSelectPin(item.pin);
      setQuery(item.pin.caption ?? item.pin.username ?? "Pin");
    }
    setResults([]);
    setIsOpen(false);
    setActiveIndex(-1);
  };

  const handleKey = (e: React.KeyboardEvent) => {
    if (!isOpen) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && activeIndex >= 0) {
      e.preventDefault();
      select(results[activeIndex]);
    } else if (e.key === "Escape") {
      setIsOpen(false);
      setActiveIndex(-1);
      inputRef.current?.blur();
    }
  };

  const clear = () => {
    selectedRef.current = false;
    setQuery("");
    setIsOpen(false);
    setResults([]);
    setActiveIndex(-1);
    inputRef.current?.focus();
  };

  let placeIdx = -1;
  let pinIdx = -1;

  return (
    <div ref={wrapperRef} className={"relative w-full min-w-0 " + (className ?? "flex-1")}>
      <div className="flex h-12 items-center gap-2 rounded-full border border-[#E0E0E0] bg-white px-4 shadow-[0_2px_6px_rgba(0,0,0,0.15)] focus-within:border-blue-500 focus-within:ring-2 focus-within:ring-blue-500/20 dark:border-zinc-700 dark:bg-zinc-900 dark:shadow-none">
        <svg className="h-4 w-4 shrink-0 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
          <circle cx="11" cy="11" r="8" />
          <path strokeLinecap="round" d="m21 21-4.35-4.35" />
        </svg>
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => {
            const q = e.target.value;
            selectedRef.current = false;
            setQuery(q);
            setActiveIndex(-1);
            if (q.trim().length < 2) {
              setResults([]);
              setIsOpen(false);
              setLoading(false);
            }
          }}
          onFocus={() => { if (results.length > 0) setIsOpen(true); }}
          onKeyDown={handleKey}
          placeholder={placeholder}
          className="flex-1 bg-transparent text-sm text-zinc-900 outline-none placeholder:text-zinc-400 dark:text-zinc-100"
          role="combobox"
          aria-expanded={isOpen}
          aria-controls="search-results-listbox"
          aria-autocomplete="list"
        />
        {query && (
          <button type="button" onClick={clear} className="rounded-full p-0.5 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300" aria-label="Clear">
            <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" d="M18 6 6 18M6 6l12 12" />
            </svg>
          </button>
        )}
        {endSlot}
      </div>

      {isOpen && (
        <ul
          role="listbox"
          id="search-results-listbox"
          className="absolute left-0 right-0 top-full z-30 mt-2 max-h-80 overflow-y-auto rounded-2xl border border-zinc-200/70 bg-white/95 py-1.5 shadow-xl shadow-zinc-900/10 backdrop-blur dark:border-zinc-700/70 dark:bg-zinc-900/95"
        >
          {loading && (
            <li className="px-4 py-3 text-sm text-zinc-400">Searching…</li>
          )}
          {!loading && results.length === 0 && (
            <li className="px-4 py-3 text-sm text-zinc-400">No results found</li>
          )}
          {!loading && results.some((r) => r.kind === "place") && (
            <>
              <li className="px-4 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
                Places
              </li>
              {results.map((item) => {
                if (item.kind !== "place" || !item.place) return null;
                placeIdx++;
                const isActive = placeIdx === activeIndex;
                return (
                  <li
                    key={`place-${item.place.id}`}
                    role="option"
                    aria-selected={isActive}
                    className={`cursor-pointer px-4 py-2.5 text-sm ${isActive ? "bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300" : "text-zinc-900 hover:bg-zinc-50 dark:text-zinc-100 dark:hover:bg-zinc-800"}`}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => select(item)}
                    onMouseEnter={() => setActiveIndex(placeIdx)}
                  >
                    <span className="line-clamp-1 font-medium">{item.place.text}</span>
                    <span className="line-clamp-1 text-xs text-zinc-500 dark:text-zinc-400">{item.place.place_name}</span>
                  </li>
                );
              })}
            </>
          )}
          {!loading && results.some((r) => r.kind === "pin") && (
            <>
              <li className="px-4 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
                Pins
              </li>
              {results.map((item) => {
                if (item.kind !== "pin" || !item.pin) return null;
                pinIdx++;
                const isActive = pinIdx === activeIndex;
                return (
                  <li
                    key={`pin-${item.pin.id}`}
                    role="option"
                    aria-selected={isActive}
                    className={`cursor-pointer px-4 py-2.5 text-sm ${isActive ? "bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300" : "text-zinc-900 hover:bg-zinc-50 dark:text-zinc-100 dark:hover:bg-zinc-800"}`}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => select(item)}
                    onMouseEnter={() => setActiveIndex(pinIdx)}
                  >
                    <span className="line-clamp-1 font-medium">
                      {item.pin.caption ?? item.pin.username ?? "Pin"}
                    </span>
                    <span className="line-clamp-1 text-xs text-zinc-500 dark:text-zinc-400">
                      {item.pin.username ? `@${item.pin.username}` : "Pin on map"}
                    </span>
                  </li>
                );
              })}
            </>
          )}
        </ul>
      )}
    </div>
  );
}
