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

const LIGHT_STYLE = `https://api.maptiler.com/maps/toner-lite/style.json?key=${process.env.NEXT_PUBLIC_MAPTILER_API_KEY}`;
const DARK_STYLE = `https://api.maptiler.com/maps/basic-v2-dark/style.json?key=${process.env.NEXT_PUBLIC_MAPTILER_API_KEY}`;

setWorkerUrl("/maplibre-gl-worker.js");

export interface MapLocation {
  lat: number;
  lng: number;
}

interface MapViewProps {
  pins: PinListEntry[];
  flyTo: { lng: number; lat: number } | null;
  highlightId: string | null;
  theme: Theme;
  onBoundsChange: (bbox: string, center: { lat: number; lng: number }) => void;
  onSelectPin: (id: string) => void;
  onMapClick: () => void;
}

interface GeoFeature {
  type: "Feature";
  geometry: { type: "Point"; coordinates: [number, number] };
  properties: {
    id: string;
    caption: string;
    username: string;
    cover: string;
  };
}

type GeoJSONLike = {
  type: "FeatureCollection";
  features: GeoFeature[];
};

const EMPTY_GEOJSON: GeoJSONLike = {
  type: "FeatureCollection",
  features: [],
};

function makePinIcon(
  size: number,
  color: string,
  ringColor: string | null,
  dotColor: string
): ImageData {
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d")!;
  const cx = size / 2;
  const cy = size * 0.3;
  const r = size * 0.26;
  const tipY = size * 0.97;

  ctx.fillStyle = color;
  ctx.strokeStyle = ringColor ?? color;
  ctx.lineWidth = size * 0.07;
  if (ringColor) {
    ctx.beginPath();
    ctx.arc(cx, cy, r + size * 0.05, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.beginPath();
    ctx.moveTo(cx, tipY);
    ctx.lineTo(cx - r * 0.85, cy + r * 0.45);
    ctx.lineTo(cx + r * 0.85, cy + r * 0.45);
    ctx.closePath();
    ctx.fill();
    ctx.stroke();
  }
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fill();
  ctx.stroke();
  ctx.beginPath();
  ctx.moveTo(cx, tipY);
  ctx.lineTo(cx - r * 0.85, cy + r * 0.45);
  ctx.lineTo(cx + r * 0.85, cy + r * 0.45);
  ctx.closePath();
  ctx.fill();
  ctx.stroke();

  ctx.fillStyle = dotColor;
  ctx.beginPath();
  ctx.arc(cx, cy, r * 0.38, 0, Math.PI * 2);
  ctx.fill();

  return ctx.getImageData(0, 0, size, size);
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
  const [hover, setHover] = useState<{
    pin: PinListEntry;
    x: number;
    y: number;
  } | null>(null);

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

    // setStyle() (theme swap) drops runtime-added sources and layers and
    // fires "style.load" again ("load" only fires once per map), but the
    // diff path can keep previously added images, so re-creation must be
    // guarded per resource.
    const ensurePinLayers = () => {
      if (!map.hasImage("pin-default")) {
        map.addImage("pin-default", makePinIcon(64, "#e11d48", null, "#ffffff"));
      }
      if (!map.hasImage("pin-selected")) {
        map.addImage(
          "pin-selected",
          makePinIcon(64, "#9f1239", "#ffffff", "#ffffff")
        );
      }

      if (!map.getSource("pins")) {
        map.addSource("pins", { type: "geojson", data: EMPTY_GEOJSON });
      }

      if (!map.getLayer("pins-base")) {
        map.addLayer({
          id: "pins-base",
          type: "symbol",
          source: "pins",
          layout: {
            "icon-image": "pin-default",
            "icon-size": 0.75,
            "icon-anchor": "bottom",
          },
        });
      }

      if (!map.getLayer("pins-selected")) {
        map.addLayer({
          id: "pins-selected",
          type: "symbol",
          source: "pins",
          layout: {
            "icon-image": "pin-selected",
            "icon-size": 1.05,
            "icon-anchor": "bottom",
          },
        });
      }
    };

    map.on("style.load", () => {
      ensurePinLayers();
      // Re-runs the pin-data and highlight-filter effects against the
      // freshly loaded style.
      setStyleVersion((v) => v + 1);
    });

    map.on("load", () => {
      if (initialized) return;
      initialized = true;

        map.on("click", pinLayers as never, (e) => {
          const id = e.features?.[0]?.properties?.id;
          if (id) onSelectRef.current(String(id));
        });
        map.on("mouseenter", pinLayers as never, () => {
          map.getCanvas().style.cursor = "pointer";
        });
        map.on("mouseleave", pinLayers as never, () => {
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
              user_id: "",
              location: "",
              geohash: "",
              caption: props.caption || null,
              category_id: 0,
              is_hidden: false,
              created_at: "",
              cover_url: props.cover || "",
              username: props.username || null,
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