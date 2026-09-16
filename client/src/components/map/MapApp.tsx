"use client";

import { useCallback, useEffect, useState } from "react";
import MapView, { type MapLocation } from "./MapView";
import CategoryBar from "@/components/pins/CategoryBar";
import PinDetailPanel from "@/components/pins/PinDetailPanel";
import CreatePinButton from "@/components/pins/CreatePinButton";
import { fetchCategories } from "@/lib/api";
import { usePins } from "@/hooks/usePins";
import { usePinDetail } from "@/hooks/usePinDetail";
import type { Category, CreatedPin, NewPinPhoto } from "@/lib/types";
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
  const [pendingLocation, setPendingLocation] = useState<MapLocation | null>(null);
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
      setPendingLocation(null);
      setDetailError(null);
    },
    [addPin]
  );

  const bannerError = categoriesError ?? pinsError ?? detailError;

  return (
    <div className="absolute inset-0">
      <MapView
        pins={pins}
        flyTo={flyTo}
        pendingLocation={pendingLocation}
        onBoundsChange={handleBoundsChange}
        onSelectPin={setSelectedPinId}
        onSelectLocation={setPendingLocation}
      />

      <div className="absolute left-0 right-0 top-0 z-10">
        <CategoryBar
          categories={categories}
          selected={selectedCategory}
          onSelect={setSelectedCategory}
        />
      </div>

      {loading && (
        <div className="absolute left-3 top-14 z-10 rounded bg-white/90 px-2 py-1 text-xs text-zinc-500 shadow">
          Loading pins…
        </div>
      )}
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

      {selectedPinId && detail && (
        <PinDetailPanel key={detail.id} pin={detail} onClose={() => setSelectedPinId(null)} />
      )}

      <CreatePinButton
        lat={pendingLocation?.lat ?? center.lat}
        lng={pendingLocation?.lng ?? center.lng}
        categories={categories}
        onCreated={handleCreated}
        onSetLocation={setPendingLocation}
      />
    </div>
  );
}