"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import MapView, { type MapLocation } from "./MapView";
import LocateButton from "./LocateButton";
import MapNavBar from "./MapNavBar";
import PinDetailPanel from "@/components/pins/PinDetailPanel";
import CreatePinButton from "@/components/pins/CreatePinButton";
import SavedPinsPanel from "@/components/pins/SavedPinsPanel";
import TrendingList, { TrendingIcon } from "@/components/pins/TrendingList";
import { usePins } from "@/hooks/usePins";
import { usePinDetail } from "@/hooks/usePinDetail";
import { useCategories } from "@/hooks/useCategories";
import { usePinStream } from "@/hooks/usePinStream";
import { useTrending } from "@/hooks/useTrending";
import { parsePoint } from "@/lib/utils";
import { useAuth } from "@/store/auth";
import { useTheme } from "@/store/theme";
import type { CreatedPin, NewPinPhoto, PinListEntry, TrendingPin } from "@/lib/types";

export default function MapApp({
  initialCategory = null,
}: {
  initialCategory?: number | null;
}) {
  const { theme } = useTheme();
  const { categories, error: categoriesError, retry: retryCategories } = useCategories();
  const [selectedCategory, setSelectedCategory] = useState<number | null>(initialCategory);
  const [selectedPinId, setSelectedPinId] = useState<string | null>(null);
  const [highlightId, setHighlightId] = useState<string | null>(null);
  const [savedOpen, setSavedOpen] = useState(false);
  const [trendingOpen, setTrendingOpen] = useState(false);
  const [bbox, setBbox] = useState<string | null>(null);
  const [center, setCenter] = useState({ lat: 17.385, lng: 78.4867 });
  const [flyTo, setFlyTo] = useState<{ lng: number; lat: number } | null>(null);
  const [postingMode, setPostingMode] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  // A category id from the URL might not exist once the category list is
  // known. Derive the effective filter instead of mutating state during an
  // effect: an unknown id falls back to "All" while the raw URL value is kept.
  const effectiveCategory =
    selectedCategory === null ||
    (categories.length > 0 && !categories.some((c) => c.id === selectedCategory))
      ? null
      : selectedCategory;

  const { pins, loading, error: pinsError, addPin } = usePins(bbox, effectiveCategory);
  const { pins: trending, loading: trendingLoading, error: trendingError } = useTrending(
    trendingOpen ? bbox : null
  );
  const reportDetailError = useCallback((message: string) => setDetailError(message), []);
  const { detail } = usePinDetail(selectedPinId, { onError: reportDetailError });

  // Realtime: new pins in the current view stream in and merge into the list.
  // A short-lived toast keeps the counter visible without stealing focus.
  const [streamToast, setStreamToast] = useState(0);
  const toastTimerRef = useRef<number | null>(null);

  const handleStreamedPin = useCallback(
    (pin: PinListEntry) => {
      addPin(pin);
      setStreamToast((n) => n + 1);
      if (toastTimerRef.current !== null) window.clearTimeout(toastTimerRef.current);
      toastTimerRef.current = window.setTimeout(() => setStreamToast(0), 4000);
    },
    [addPin]
  );

  usePinStream(bbox, effectiveCategory, handleStreamedPin);

  useEffect(() => {
    return () => {
      if (toastTimerRef.current !== null) window.clearTimeout(toastTimerRef.current);
    };
  }, []);

  const router = useRouter();
  const pathname = usePathname();

  const handleSelectCategory = useCallback(
    (id: number | null) => {
      setSelectedCategory(id);
      router.replace(id === null ? pathname : `${pathname}?category=${id}`, {
        scroll: false,
      });
    },
    [pathname, router]
  );

  // If the URL carried a category id that no longer exists, clean the URL
  // (the effective filter already falls back to "All"). External-system sync
  // via router.replace is fine inside an effect; state is not reset here.
  useEffect(() => {
    if (
      categories.length > 0 &&
      selectedCategory !== null &&
      !categories.some((c) => c.id === selectedCategory)
    ) {
      router.replace(pathname, { scroll: false });
    }
  }, [categories, selectedCategory, pathname, router]);

  // The overlays (posting crosshair, create dialog, saved panel) are mutually
  // exclusive with pin selection: every "navigate somewhere" action closes the
  // others through this one reset.
  const closeOverlays = useCallback(() => {
    setPostingMode(false);
    setCreateOpen(false);
    setSavedOpen(false);
    setTrendingOpen(false);
    setDetailError(null);
  }, []);

  const handleBoundsChange = useCallback(
    (bboxValue: string, c: { lat: number; lng: number }) => {
      setBbox(bboxValue);
      setCenter(c);
      setFlyTo(null);
    },
    []
  );

  const handleMapClick = useCallback(() => {
    if (createOpen) {
      setCreateOpen(false);
      setPostingMode(false);
      return;
    }
    setPostingMode((p) => !p);
    setSelectedPinId(null);
    setHighlightId(null);
    setSavedOpen(false);
    setTrendingOpen(false);
    setDetailError(null);
  }, [createOpen]);

  const handleSelectPin = useCallback((id: string) => {
    setSelectedPinId(id);
    setHighlightId(id);
    closeOverlays();
  }, [closeOverlays]);

  const handleSetLocation = useCallback((location: MapLocation) => {
    setFlyTo(location);
  }, []);

  const handleCreated = useCallback(
    (pin: CreatedPin, photos: NewPinPhoto[]) => {
      const cover = photos[0]?.thumbnail_url ?? photos[0]?.photo_url ?? "";
      const username = useAuth.getState().user?.username ?? "";
      addPin({
        id: pin.id,
        user_id: pin.user_id,
        location: pin.location,
        geohash: pin.geohash,
        caption: pin.caption,
        category_id: pin.category_id,
        is_hidden: pin.is_hidden,
        views: pin.views,
        created_at: pin.created_at,
        cover_url: cover,
        username,
      });
      setSelectedPinId(pin.id);
      setHighlightId(pin.id);
      closeOverlays();
      const point = parsePoint(pin.location);
      if (point) setFlyTo(point);
    },
    [addPin, closeOverlays]
  );

  const handleSearchPin = useCallback((entry: PinListEntry) => {
    const point = parsePoint(entry.location);
    if (point) setFlyTo(point);
    setSelectedPinId(entry.id);
    setHighlightId(entry.id);
    closeOverlays();
  }, [closeOverlays]);

  const handleSearchPlace = useCallback(
    (c: { lat: number; lng: number }) => {
      setFlyTo(c);
      closeOverlays();
    },
    [closeOverlays]
  );

  const handleLocate = useCallback((c: { lat: number; lng: number }) => {
    setFlyTo(c);
    closeOverlays();
  }, [closeOverlays]);

  const handleOpenSaved = useCallback(() => {
    setSelectedPinId(null);
    closeOverlays();
    setSavedOpen(true);
  }, [closeOverlays]);

  const handleOpenSavedPin = useCallback((entry: PinListEntry) => {
    setHighlightId(entry.id);
    setSelectedPinId(null);
    closeOverlays();
    const point = parsePoint(entry.location);
    if (point) setFlyTo(point);
  }, [closeOverlays]);

  const handleOpenTrending = useCallback(
    (entry: TrendingPin) => {
      setTrendingOpen(false);
      const point = parsePoint(entry.location);
      if (point) setFlyTo(point);
      setSelectedPinId(entry.id);
      setHighlightId(entry.id);
      closeOverlays();
    },
    [closeOverlays]
  );

  const bannerError = categoriesError ?? pinsError ?? detailError;
  const activeCategory = categories.find((c) => c.id === effectiveCategory) ?? null;

  return (
    <div className="absolute inset-0">
      <MapView
        pins={pins}
        flyTo={flyTo}
        highlightId={highlightId}
        theme={theme}
        onBoundsChange={handleBoundsChange}
        onSelectPin={handleSelectPin}
        onMapClick={handleMapClick}
      />

      {postingMode && (
        <div
          className="pointer-events-none absolute left-1/2 top-1/2 z-10 flex -translate-x-1/2 -translate-y-1/2 items-center justify-center"
          aria-hidden
        >
          <div className="absolute h-0.5 w-16 bg-rose-600 shadow-[0_0_4px_rgba(255,255,255,0.9)]" />
          <div className="absolute h-16 w-0.5 bg-rose-600 shadow-[0_0_4px_rgba(255,255,255,0.9)]" />
        </div>
      )}

      <MapNavBar
        center={center}
        onSelectPlace={handleSearchPlace}
        onSelectPin={handleSearchPin}
        categories={categories}
        selectedCategory={effectiveCategory}
        onSelectCategory={handleSelectCategory}
        onOpenSaved={handleOpenSaved}
      />

      {loading && (
        <div className="absolute left-3 top-36 z-10 rounded bg-white/90 px-2 py-1 text-xs text-zinc-500 shadow dark:bg-zinc-900/90 dark:text-zinc-400">
          Loading pins…
        </div>
      )}
      <div className="absolute bottom-20 right-4 z-10">
        <LocateButton onLocate={handleLocate} />
      </div>
      {bannerError && (
        <div
          role="alert"
          className="absolute left-3 top-40 z-10 rounded bg-rose-50 px-2 py-1 text-xs text-rose-600 shadow dark:bg-rose-950/60 dark:text-rose-300"
        >
          {bannerError}
          {categoriesError && (
            <button
              type="button"
              onClick={retryCategories}
              className="ml-2 font-semibold underline"
            >
              Retry
            </button>
          )}
        </div>
      )}

      {!loading && !bannerError && pins.length > 0 && (
        <div className="pointer-events-none absolute bottom-6 left-4 z-10 rounded-full bg-white/90 px-4 py-2 text-xs font-medium text-zinc-500 shadow ring-1 ring-zinc-200/70 backdrop-blur dark:bg-zinc-900/90 dark:text-zinc-400 dark:ring-zinc-700/70">
          {pins.length} {activeCategory ? `${activeCategory.name} places` : "places"} here
        </div>
      )}

      {!trendingOpen ? (
        <button
          type="button"
          onClick={() => setTrendingOpen(true)}
          aria-expanded={false}
          className="absolute bottom-24 left-4 z-10 flex items-center gap-1.5 rounded-full bg-white/90 px-3.5 py-2 text-xs font-semibold text-zinc-700 shadow ring-1 ring-zinc-200/70 backdrop-blur transition-colors hover:bg-white dark:bg-zinc-900/90 dark:text-zinc-200 dark:ring-zinc-700/70 dark:hover:bg-zinc-900"
        >
          <TrendingIcon className="h-4 w-4 text-rose-500" />
          Trending
        </button>
      ) : (
        <TrendingList
          pins={trending}
          loading={trendingLoading}
          error={trendingError}
          onClose={() => setTrendingOpen(false)}
          onOpenPin={handleOpenTrending}
        />
      )}

      {streamToast > 0 && (
        <button
          type="button"
          onClick={() => setStreamToast(0)}
          className="absolute bottom-16 left-1/2 z-20 -translate-x-1/2 rounded-full bg-zinc-900/90 px-4 py-2 text-xs font-medium text-white shadow-lg backdrop-blur transition-opacity hover:opacity-90 dark:bg-zinc-100/90 dark:text-zinc-900"
        >
          {streamToast} new pin{streamToast > 1 ? "s" : ""} in view
          <span className="ml-1.5 opacity-70">dismiss</span>
        </button>
      )}

      {!loading && !bannerError && pins.length === 0 && (
        <div className="absolute bottom-6 left-4 z-10 rounded-full bg-white/90 px-4 py-2 text-xs font-medium text-zinc-500 shadow ring-1 ring-zinc-200/70 backdrop-blur dark:bg-zinc-900/90 dark:text-zinc-400 dark:ring-zinc-700/70">
          {activeCategory ? `No ${activeCategory.name} places here` : "No pins in this area yet"}
        </div>
      )}

      {selectedPinId && detail && (
        <PinDetailPanel
          key={detail.id}
          pin={detail}
          onClose={() => setSelectedPinId(null)}
        />
      )}

      {savedOpen && (
        <SavedPinsPanel
          activeId={highlightId}
          onClose={() => setSavedOpen(false)}
          onOpenPin={handleOpenSavedPin}
        />
      )}

      <CreatePinButton
        posting={postingMode}
        open={createOpen}
        onOpenChange={setCreateOpen}
        lat={center.lat}
        lng={center.lng}
        categories={categories}
        onCreated={handleCreated}
        onSetLocation={handleSetLocation}
      />
    </div>
  );
}
