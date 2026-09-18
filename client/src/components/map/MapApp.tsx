"use client";

import { useCallback, useEffect, useState } from "react";
import MapView, { type MapLocation } from "./MapView";
import LocateButton from "./LocateButton";
import MapNavBar from "./MapNavBar";
import PinDetailPanel from "@/components/pins/PinDetailPanel";
import CreatePinButton from "@/components/pins/CreatePinButton";
import SavedPinsPanel from "@/components/pins/SavedPinsPanel";
import { fetchCategories, type FavoriteEntry } from "@/lib/api";
import { usePins } from "@/hooks/usePins";
import { usePinDetail } from "@/hooks/usePinDetail";
import type { Category, CreatedPin, NewPinPhoto, PinListEntry } from "@/lib/types";
import { parsePoint } from "@/lib/utils";
import { useAuth } from "@/store/auth";
import { useTheme } from "@/store/theme";

export default function MapApp() {
  const { theme } = useTheme();
  const [categories, setCategories] = useState<Category[]>([]);
  const [categoriesError, setCategoriesError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [selectedPinId, setSelectedPinId] = useState<string | null>(null);
  const [highlightId, setHighlightId] = useState<string | null>(null);
  const [savedOpen, setSavedOpen] = useState(false);
  const [bbox, setBbox] = useState<string | null>(null);
  const [center, setCenter] = useState({ lat: 17.385, lng: 78.4867 });
  const [flyTo, setFlyTo] = useState<{ lng: number; lat: number } | null>(null);
  const [postingMode, setPostingMode] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

const { pins, loading, error: pinsError, addPin } = usePins(bbox, selectedCategory);
  const reportDetailError = useCallback((message: string) => setDetailError(message), []);
const { detail } = usePinDetail(selectedPinId, { onError: reportDetailError });

  const loadCategories = useCallback(async () => {
    try {
      setCategories(await fetchCategories());
    } catch (e) {
      setCategoriesError(e instanceof Error ? e.message : "something went wrong");
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    fetchCategories()
      .then((data) => {
        if (!cancelled) setCategories(data);
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          setCategoriesError(
            e instanceof Error ? e.message : "something went wrong"
          );
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const retryCategories = useCallback(() => {
    setCategoriesError(null);
    void loadCategories();
  }, [loadCategories]);

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
    setSelectedPinId((id) => {
      if (id) setDetailError(null);
      return null;
    });
    setHighlightId(null);
    setSavedOpen(false);
  }, [createOpen]);

  const handleSelectPin = useCallback((id: string) => {
    setSelectedPinId(id);
    setHighlightId(id);
    setPostingMode(false);
    setCreateOpen(false);
    setSavedOpen(false);
  }, []);

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
        created_at: pin.created_at,
        cover_url: cover,
        username,
      });
      setSelectedPinId(pin.id);
      setHighlightId(pin.id);
      setSavedOpen(false);
      const point = parsePoint(pin.location);
      if (point) setFlyTo(point);
      setPostingMode(false);
      setCreateOpen(false);
      setDetailError(null);
    },
    [addPin]
  );

  const handleSearchPin = useCallback((entry: PinListEntry) => {
    const point = parsePoint(entry.location);
    if (point) setFlyTo(point);
    setSelectedPinId(entry.id);
    setHighlightId(entry.id);
    setPostingMode(false);
    setCreateOpen(false);
    setSavedOpen(false);
  }, []);

  const handleSearchPlace = useCallback(
    (c: { lat: number; lng: number }) => {
      setFlyTo(c);
      setPostingMode(false);
      setCreateOpen(false);
      setSavedOpen(false);
    },
    []
  );

  const handleLocate = useCallback((c: { lat: number; lng: number }) => {
    setFlyTo(c);
    setPostingMode(false);
    setCreateOpen(false);
    setSavedOpen(false);
  }, []);

  const handleOpenSaved = useCallback(() => {
    setSelectedPinId(null);
    setSavedOpen(true);
  }, []);

  const handleOpenSavedPin = useCallback((entry: FavoriteEntry) => {
    setHighlightId(entry.id);
    setSelectedPinId(null);
    const point = parsePoint(entry.location);
    if (point) setFlyTo(point);
    setPostingMode(false);
    setCreateOpen(false);
  }, []);

  const bannerError = categoriesError ?? pinsError ?? detailError;

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
          <div className="absolute h-2.5 w-2.5 rounded-full border-2 border-rose-600" />
        </div>
      )}

      <MapNavBar
        center={center}
        onSelectPlace={handleSearchPlace}
        onSelectPin={handleSearchPin}
        categories={categories}
        selectedCategory={selectedCategory}
        onSelectCategory={setSelectedCategory}
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

      {!loading && !bannerError && pins.length === 0 && (
        <div className="absolute bottom-6 left-4 z-10 rounded-full bg-white/90 px-4 py-2 text-xs font-medium text-zinc-500 shadow ring-1 ring-zinc-200/70 backdrop-blur dark:bg-zinc-900/90 dark:text-zinc-400 dark:ring-zinc-700/70">
          No pins in this area yet
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