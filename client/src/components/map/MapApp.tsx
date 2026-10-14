"use client";

import { useCallback, useState } from "react";
import MapView, { type MapLocation } from "./MapView";
import LocateButton from "./LocateButton";
import MapNavBar from "./MapNavBar";
import { PinsNotification } from "./PinsNotification";
import { PostingCrosshair } from "./PostingCrosshair";
import PinDetailPanel from "@/components/pins/PinDetailPanel";
import CreatePinButton from "@/components/pins/CreatePinButton";
import SavedPinsPanel from "@/components/pins/SavedPinsPanel";
import TrendingList, { TrendingIcon } from "@/features/social/components/TrendingList";
import { usePins } from "@/hooks/usePins";
import { usePinDetail } from "@/hooks/usePinDetail";
import { useMapCategoryFilter } from "@/hooks/useMapCategoryFilter";
import { useDebouncedBbox } from "@/hooks/useDebouncedBbox";
import { useStreamToast } from "@/hooks/useStreamToast";
import { usePinStream } from "@/hooks/usePinStream";
import { useTrending } from "@/features/social/hooks/useTrending";
import { parsePoint } from "@/lib/utils/geo";
import { toPinListEntry } from "@/lib/utils/pinEntry";
import { useAuth } from "@/store/auth";
import { useTheme } from "@/store/theme";
import type { CreatedPin, NewPinPhoto, PinListEntry, TrendingPin } from "@/shared/types/pin";

export default function MapApp({
  initialCategory = null,
}: {
  initialCategory?: string | null;
}) {
  const { theme } = useTheme();
  const { categories, categoriesError, retryCategories, effectiveCategory, handleSelectCategory } =
    useMapCategoryFilter(initialCategory);
  const [selectedPinId, setSelectedPinId] = useState<string | null>(null);
  const [highlightId, setHighlightId] = useState<string | null>(null);
  const [savedOpen, setSavedOpen] = useState(false);
  const [trendingOpen, setTrendingOpen] = useState(false);
  const { bbox, updateBbox } = useDebouncedBbox();
  const [center, setCenter] = useState({ lat: 17.385, lng: 78.4867 });
  const [flyTo, setFlyTo] = useState<{ lng: number; lat: number } | null>(null);
  const [postingMode, setPostingMode] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  const { pins, loading, error: pinsError, addPin, removePin, refetch } = usePins(bbox, effectiveCategory);
  const { pins: trending, loading: trendingLoading, error: trendingError } = useTrending(
    trendingOpen ? bbox : null
  );
  const reportDetailError = useCallback((message: string) => setDetailError(message), []);
  const { detail } = usePinDetail(selectedPinId, { onError: reportDetailError });

  // Realtime: new pins in the current view stream in and merge into the list.
  // A short-lived toast keeps the counter visible without stealing focus.
  const { count: streamToast, bump: bumpStreamToast, dismiss: dismissStreamToast } =
    useStreamToast();

  const handleStreamedPin = useCallback(
    (pin: PinListEntry) => {
      addPin(pin);
      bumpStreamToast();
    },
    [addPin, bumpStreamToast]
  );

  // Realtime removals: drop the pin from the map list and close the detail
  // panel when its open pin disappears (owner delete or report approve).
  const handleStreamedRemoval = useCallback(
    (id: string) => {
      removePin(id);
      setSelectedPinId((prev) => (prev === id ? null : prev));
      setHighlightId((prev) => (prev === id ? null : prev));
    },
    [removePin]
  );

  const handleStreamReconnect = useCallback(() => {
    refetch();
  }, [refetch]);

  usePinStream(bbox, effectiveCategory, handleStreamedPin, handleStreamedRemoval, handleStreamReconnect);

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
      setCenter(c);
      setFlyTo(null);
      updateBbox(bboxValue);
    },
    [updateBbox]
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
      const username = useAuth.getState().user?.username ?? "";
      addPin(toPinListEntry(pin, photos, username));
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

      {postingMode && <PostingCrosshair />}

      <MapNavBar
        center={center}
        onSelectPlace={handleSearchPlace}
        onSelectPin={handleSearchPin}
        categories={categories}
        selectedCategory={effectiveCategory}
        onSelectCategory={handleSelectCategory}
        onOpenSaved={handleOpenSaved}
      />

      <PinsNotification
        loading={loading}
        error={bannerError}
        showRetry={categoriesError !== null}
        onRetry={retryCategories}
      />

      <div className="absolute bottom-20 right-4 z-10">
        <LocateButton onLocate={handleLocate} />
      </div>

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
          onClick={dismissStreamToast}
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
          onDeleted={(id) => {
            removePin(id);
            setSelectedPinId(null);
          }}
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
