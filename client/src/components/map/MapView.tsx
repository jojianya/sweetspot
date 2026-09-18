"use client";

import { useEffect, useRef, useState } from "react";
import {
  Map as MapLibreMap,
  Marker,
  setWorkerUrl,
  type GeoJSONSource,
  type Map as MaplibreMap,
  type Marker as MaplibreMarker,
} from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import type { PinListEntry } from "@/lib/types";
import { boundsToValidBbox, parsePoint } from "@/lib/utils";
import { useGeolocation } from "@/hooks/useGeolocation";
import type { Theme } from "@/store/theme";
import { ensurePinLayers, type GeoFeature } from "./pinLayers";

const LIGHT_STYLE = `https://api.maptiler.com/maps/toner-lite/style.json?key=${process.env.NEXT_PUBLIC_MAPTILER_API_KEY}`;
const DARK_STYLE = `https://api.maptiler.com/maps/basic-v2-dark/style.json?key=${process.env.NEXT_PUBLIC_MAPTILER_API_KEY}`;

setWorkerUrl("/maplibre-gl-worker.js");

export interface MapLocation {
  lat: number;
  lng: number;
}

/** Data carried by a hover tooltip; only fields actually rendered. */
type PinHover = {
  id: string;
  caption: string | null;
  username: string | null;
  cover_url: string;
};

interface MapViewProps {
  pins: PinListEntry[];
  flyTo: { lng: number; lat: number } | null;
  highlightId: string | null;
  theme: Theme;
  onBoundsChange: (bbox: string, center: { lat: number; lng: number }) => void;
  onSelectPin: (id: string) => void;
  onMapClick: () => void;
}

export default function MapView({
  pins,
  flyTo,
  highlightId,
  theme,
  onBoundsChange,
  onSelectPin,
  onMapClick,
}: MapViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MaplibreMap | null>(null);
  const userMarkerRef = useRef<MaplibreMarker | null>(null);
  const { position } = useGeolocation();
  const [styleReady, setStyleReady] = useState(false);
  const [styleVersion, setStyleVersion] = useState(0);
  const themeRef = useRef(theme);
  const [hover, setHover] = useState<{ pin: PinHover; x: number; y: number } | null>(
    null
  );

  const onBoundsRef = useRef(onBoundsChange);
  const onSelectRef = useRef(onSelectPin);
  const onMapClickRef = useRef(onMapClick);

  useEffect(() => {
    onBoundsRef.current = onBoundsChange;
    onSelectRef.current = onSelectPin;
    onMapClickRef.current = onMapClick;
  });

  useEffect(() => {
    const map = new MapLibreMap({
      container: containerRef.current!,
      style: theme === "dark" ? DARK_STYLE : LIGHT_STYLE,
      zoom: 10,
      minZoom: 5,
      maxZoom: 18,
    });
    mapRef.current = map;

    const pinLayers = ["pins-base", "pins-selected"];
    let initialized = false;

    map.on("style.load", () => {
      ensurePinLayers(map);
      // Re-runs the pin-data and highlight-filter effects against the
      // freshly loaded style.
      setStyleVersion((v) => v + 1);
    });

    map.on("load", () => {
      if (initialized) return;
      initialized = true;

        // Typed overload: MapLibre v6 accepts string[] layer ids directly.
        map.on("click", pinLayers, (e) => {
          const id = e.features?.[0]?.properties?.id;
          if (id) onSelectRef.current(String(id));
        });
        map.on("mouseenter", pinLayers, () => {
          map.getCanvas().style.cursor = "pointer";
        });
        map.on("mouseleave", pinLayers, () => {
          map.getCanvas().style.cursor = "";
          setHover(null);
        });
        map.on("mousemove", "pins-base", (e) => {
          const feature = e.features?.[0];
          const props = feature?.properties;
          if (!props?.id) {
            setHover(null);
            return;
          }
          setHover({
            pin: {
              id: String(props.id),
              caption: props.caption || null,
              username: props.username || null,
              cover_url: props.cover || "",
            },
            x: e.point.x,
            y: e.point.y,
          });
        });

        map.on("click", (e) => {
          const hit = map.queryRenderedFeatures(e.point, { layers: pinLayers });
          if (hit.length > 0) return;
          onMapClickRef.current();
        });

        const [south, west, north, east] = boundsToValidBbox(map.getBounds());
        onBoundsRef.current(`${south},${west},${north},${east}`, {
          lat: map.getCenter().lat,
          lng: map.getCenter().lng,
        });
        setStyleReady(true);
    });

    map.on("moveend", () => {
      const [south, west, north, east] = boundsToValidBbox(map.getBounds());
      onBoundsRef.current(`${south},${west},${north},${east}`, {
        lat: map.getCenter().lat,
        lng: map.getCenter().lng,
      });
    });

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    if (themeRef.current === theme) return;
    themeRef.current = theme;
    map.setStyle(theme === "dark" ? DARK_STYLE : LIGHT_STYLE);
  }, [theme]);

  useEffect(() => {
    const map = mapRef.current;
    if (!position || !map) return;
    map.jumpTo({
      center: [position.lng, position.lat],
      zoom: Math.max(map.getZoom(), 12),
    });
    if (userMarkerRef.current) {
      userMarkerRef.current.setLngLat([position.lng, position.lat]);
      return;
    }
    const el = document.createElement("div");
    el.className = "h-4 w-4 rounded-full bg-blue-500 shadow-md ring-4 ring-white";
    userMarkerRef.current = new Marker({ element: el })
      .setLngLat([position.lng, position.lat])
      .addTo(map);
    return () => {
      userMarkerRef.current?.remove();
      userMarkerRef.current = null;
    };
  }, [position]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !styleReady) return;

    const source = map.getSource("pins");
    if (!source) return;

    const features: GeoFeature[] = [];
    for (const p of pins) {
      const point = parsePoint(p.location);
      if (!point) continue;
      features.push({
        type: "Feature",
        geometry: { type: "Point", coordinates: [point.lng, point.lat] },
        properties: {
          id: p.id,
          caption: p.caption ?? "",
          username: p.username ?? "",
          cover: p.cover_url,
        },
      });
    }

    (source as GeoJSONSource).setData({ type: "FeatureCollection", features });
  }, [pins, styleReady, styleVersion]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !flyTo) return;
    map.flyTo({
      center: [flyTo.lng, flyTo.lat],
      zoom: Math.max(map.getZoom(), 13),
    });
  }, [flyTo]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !styleReady) return;
    const hasBase = map.getLayer("pins-base");
    const hasSelected = map.getLayer("pins-selected");
    if (!hasBase || !hasSelected) return;

    map.setFilter(
      "pins-base",
      highlightId ? ["!=", ["get", "id"], highlightId] : ["all"]
    );
    map.setFilter(
      "pins-selected",
      highlightId ? ["==", ["get", "id"], highlightId] : ["all"]
    );
  }, [highlightId, styleReady, styleVersion]);

  return (
    <div className="relative h-full w-full">
      <div ref={containerRef} className="h-full w-full" />
      {hover && hover.pin.cover_url && (
        <div
          className="pointer-events-none absolute z-20 -translate-x-1/2 -translate-y-[calc(100%+12px)]"
          style={{ left: hover.x + 18, top: hover.y }}
        >
          <div className="flex w-56 items-center gap-2.5 rounded-xl bg-white/95 p-2 shadow-lg ring-1 ring-zinc-200 backdrop-blur dark:bg-zinc-900/95 dark:ring-zinc-700">
            <img
              src={hover.pin.cover_url}
              alt=""
              className="h-11 w-11 shrink-0 rounded-lg object-cover"
            />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                {hover.pin.caption ?? "Untitled"}
              </p>
              {hover.pin.username && (
                <p className="truncate text-xs text-zinc-500 dark:text-zinc-400">
                  @{hover.pin.username}
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
