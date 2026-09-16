"use client";

import { useCallback, useEffect, useState } from "react";
import MapView, { type MapLocation } from "./MapView";
import SearchBar from "./SearchBar";
import LocateButton from "./LocateButton";
import CategoryBar from "@/components/pins/CategoryBar";
import PinDetailPanel from "@/components/pins/PinDetailPanel";
import CreatePinButton from "@/components/pins/CreatePinButton";
import { fetchCategories, deletePin } from "@/lib/api";
import { usePins } from "@/hooks/usePins";
import { usePinDetail } from "@/hooks/usePinDetail";
import type { Category, CreatedPin, NewPinPhoto, PinListEntry } from "@/lib/types";
import { parsePoint } from "@/lib/utils";
import { useAuth } from "@/store/auth";

export default function MapApp() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [categoriesError, setCategoriesError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [selectedPinId, setSelectedPinId] = useState<string | null>(null);
  const [bbox, setBbox] = useState<string | null>(null);
  const [center, setCenter] = useState({ lat: 17.385, lng: 78.4867 });
  const [flyTo, setFlyTo] = useState<{ lng: number; lat: number } | null>(null);
  const [postingMode, setPostingMode] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

const { pins, loading, error: pinsError, addPin, removePin } = usePins(bbox, selectedCategory);
const { user } = useAuth();
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
    setPostingMode((p) => !p);
    setSelectedPinId((id) => {
      if (id) setDetailError(null);
      return null;
    });
  }, []);

  const handleSelectPin = useCallback((id: string) => {
    setSelectedPinId(id);
    setPostingMode(false);
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
      const point = parsePoint(pin.location);
      if (point) setFlyTo(point);
      setPostingMode(false);
      setDetailError(null);
    },
    [addPin]
  );

  const handleSearchPin = useCallback((entry: PinListEntry) => {
    const point = parsePoint(entry.location);
    if (point) setFlyTo(point);
    setSelectedPinId(entry.id);
    setPostingMode(false);
  }, []);

  const handleSearchPlace = useCallback(
    (c: { lat: number; lng: number }) => {
      setFlyTo(c);
      setPostingMode(false);
    },
    []
  );

  const handleLocate = useCallback((c: { lat: number; lng: number }) => {
    setFlyTo(c);
    setPostingMode(false);
  }, []);

  const handleDeletePin = useCallback(
    async (id: string) => {
      await deletePin(id);
      removePin(id);
      setSelectedPinId(null);
    },
    [removePin]
  );

  const bannerError = categoriesError ?? pinsError ?? detailError;

  return (
    <div className="absolute inset-0">
      <MapView
        pins={pins}
        flyTo={flyTo}
        selectedPinId={selectedPinId}
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

      <div className="absolute left-0 right-0 top-0 z-10 flex flex-col items-center gap-1 px-4 pt-2.5">
        <SearchBar
          center={center}
          onSelectPlace={handleSearchPlace}
          onSelectPin={handleSearchPin}
        />
        <div className="w-full rounded-2xl bg-white/70 px-1 py-1 shadow-lg shadow-zinc-900/5 backdrop-blur ring-1 ring-zinc-200/60">
          <CategoryBar
            categories={categories}
            selected={selectedCategory}
            onSelect={setSelectedCategory}
          />
        </div>
      </div>

      {loading && (
        <div className="absolute left-3 top-14 z-10 rounded bg-white/90 px-2 py-1 text-xs text-zinc-500 shadow">
          Loading pins…
        </div>
      )}
      <div className="absolute bottom-20 right-4 z-10">
        <LocateButton onLocate={handleLocate} />
      </div>
      {bannerError && (
        <div
          role="alert"
          className="absolute left-3 top-20 z-10 rounded bg-rose-50 px-2 py-1 text-xs text-rose-600 shadow"
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
        <div className="absolute bottom-6 left-4 z-10 rounded-full bg-white/90 px-4 py-2 text-xs font-medium text-zinc-500 shadow ring-1 ring-zinc-200/70 backdrop-blur">
          No pins in this area yet
        </div>
      )}

      {selectedPinId && detail && (
        <PinDetailPanel
          key={detail.id}
          pin={detail}
          currentUserId={user?.id ?? null}
          onDelete={handleDeletePin}
          onClose={() => setSelectedPinId(null)}
        />
      )}

      {postingMode && (
        <CreatePinButton
          lat={center.lat}
          lng={center.lng}
          categories={categories}
          onCreated={handleCreated}
          onSetLocation={handleSetLocation}
        />
      )}
    </div>
  );
}