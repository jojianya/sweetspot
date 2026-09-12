"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import MapView from "@/components/MapView";
import CategoryBar from "@/components/CategoryBar";
import PinDetailPanel from "@/components/PinDetailPanel";
import CreatePinButton from "@/components/CreatePinButton";
import {
  fetchCategories,
  fetchPin,
  fetchPins,
  type CreatedPin,
} from "@/lib/api";
import type { NewPinPhoto, PinDetail, PinListEntry } from "@/lib/types";
import { parsePoint } from "@/lib/format";
import { useAuth } from "@/store/auth";

export default function MapApp() {
  const [pins, setPins] = useState<PinListEntry[]>([]);
  const [categories, setCategories] = useState<{ id: number; name: string }[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(null);
  const [selectedPinId, setSelectedPinId] = useState<string | null>(null);
  const [detail, setDetail] = useState<PinDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [center, setCenter] = useState({ lat: 17.385, lng: 78.4867 });
  const [flyTo, setFlyTo] = useState<{ lng: number; lat: number } | null>(null);

  const bboxRef = useRef<string | null>(null);
  const pendingRef = useRef(0);

  const loadPins = useCallback(async () => {
    const bbox = bboxRef.current;
    if (!bbox) return;
    const req = ++pendingRef.current;
    setLoading(true);
    setError(null);
    try {
      const data = await fetchPins(bbox, selectedCategory, useAuth.getState().token);
      if (req === pendingRef.current) setPins(data);
    } catch (e) {
      if (req === pendingRef.current) setError((e as Error).message);
    } finally {
      if (req === pendingRef.current) setLoading(false);
    }
  }, [selectedCategory]);

  useEffect(() => {
    fetchCategories()
      .then(setCategories)
      .catch((e) => setError((e as Error).message));
  }, []);

  useEffect(() => {
    loadPins();
  }, [loadPins]);

  const handleBoundsChange = useCallback(
    (bbox: string, c: { lat: number; lng: number }) => {
      bboxRef.current = bbox;
      setCenter(c);
      setFlyTo(null);
      void loadPins();
    },
    [loadPins]
  );

  const openDetail = useCallback(async (id: string) => {
    setSelectedPinId(id);
    setDetail(null);
    try {
      const d = await fetchPin(id, useAuth.getState().token);
      setDetail(d);
    } catch (e) {
      setError((e as Error).message);
    }
  }, []);

  const handleCreated = useCallback(
    (pin: CreatedPin, photos: NewPinPhoto[]) => {
      const cover = photos[0]?.thumbnail_url ?? photos[0]?.photo_url ?? "";
      const username = useAuth.getState().user?.username ?? "";
      setPins((prev) =>
        prev.some((p) => p.id === pin.id)
          ? prev
          : [
              {
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
              },
              ...prev,
            ]
      );
      openDetail(pin.id);
      const { lng, lat } = parsePoint(pin.location);
      setFlyTo({ lng, lat });
    },
    [openDetail]
  );

  return (
    <div className="absolute inset-0">
      <MapView
        pins={pins}
        flyTo={flyTo}
        onBoundsChange={handleBoundsChange}
        onSelectPin={openDetail}
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
      {error && (
        <div
          role="alert"
          className="absolute left-3 top-20 z-10 rounded bg-rose-50 px-2 py-1 text-xs text-rose-600 shadow"
        >
          {error}
        </div>
      )}

      {selectedPinId && detail && (
        <PinDetailPanel pin={detail} onClose={() => setSelectedPinId(null)} />
      )}

      <CreatePinButton
        lat={center.lat}
        lng={center.lng}
        categories={categories}
        onCreated={handleCreated}
      />
    </div>
  );
}