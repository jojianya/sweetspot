"use client";

import { useEffect, useRef, useState } from "react";
import {
  LngLatBounds,
  Map as MapLibreMap,
  Marker,
  setWorkerUrl,
  type ExpressionSpecification,
  type GeoJSONSource,
  type Map as MaplibreMap,
  type Marker as MaplibreMarker,
} from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import type { PinListEntry } from "@/lib/types";
import {
  boundsToValidBbox,
  parsePoint,
  selectNearbyPins,
  type PinCoordinate,
} from "@/lib/utils";
import { useGeolocation } from "@/hooks/useGeolocation";
import type { Theme } from "@/store/theme";
import { ensurePinLayers, notCluster, type GeoFeature } from "./pinLayers";

const MAPTILER_KEY = process.env.NEXT_PUBLIC_MAPTILER_API_KEY;

const LIGHT_STYLE = `https://api.maptiler.com/maps/toner-lite/style.json?key=${MAPTILER_KEY}`;
const DARK_STYLE = `https://api.maptiler.com/maps/basic-v2-dark/style.json?key=${MAPTILER_KEY}`;

setWorkerUrl("/maplibre-gl-worker.js");

export interface MapLocation {
  lat: number;
  lng: number;
}

const PIN_FIT_MAX_ZOOM = 15;
const PIN_FIT_MIN_SPAN_DEGREES = 0.01;
const PIN_FIT_DURATION_MS = 650;
const PIN_FIT_PADDING = {
  top: 96,
  right: 96,
  bottom: 160,
  left: 96,
};

function fitPinNeighborhood(
  map: MaplibreMap,
  pins: readonly PinListEntry[],
  clickedId: string
): void {
  const points: PinCoordinate[] = selectNearbyPins(clickedId, pins);
  if (points.length < 2) return;

  let bounds = new LngLatBounds();
  for (const point of points) {
    bounds.extend([point.lng, point.lat]);
  }

  // Avoid asking fitBounds to maximize zoom when pins share (or nearly share)
  // a coordinate. A small geographic floor keeps the detail view readable.
  const west = bounds.getWest();
  const east = bounds.getEast();
  const south = bounds.getSouth();
  const north = bounds.getNorth();
  const centerLng = (west + east) / 2;
  const centerLat = (south + north) / 2;
  const lngSpan = Math.max(east - west, PIN_FIT_MIN_SPAN_DEGREES);
  const latSpan = Math.max(north - south, PIN_FIT_MIN_SPAN_DEGREES);

  if (
    east - west < PIN_FIT_MIN_SPAN_DEGREES ||
    north - south < PIN_FIT_MIN_SPAN_DEGREES
  ) {
    bounds = new LngLatBounds(
      [centerLng - lngSpan / 2, centerLat - latSpan / 2],
      [centerLng + lngSpan / 2, centerLat + latSpan / 2]
    );
  }

  map.fitBounds(bounds, {
    padding: PIN_FIT_PADDING,
    maxZoom: PIN_FIT_MAX_ZOOM,
    duration: PIN_FIT_DURATION_MS,
    bearing: map.getBearing(),
  });
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
  const [clusterHover, setClusterHover] = useState<{ count: number; x: number; y: number } | null>(
    null
  );

  const onBoundsRef = useRef(onBoundsChange);
  const onSelectRef = useRef(onSelectPin);
  const onMapClickRef = useRef(onMapClick);
  const pinsRef = useRef(pins);

  useEffect(() => {
    onBoundsRef.current = onBoundsChange;
    onSelectRef.current = onSelectPin;
    onMapClickRef.current = onMapClick;
  });

  useEffect(() => {
    pinsRef.current = pins;
  }, [pins]);

  useEffect(() => {
    if (!MAPTILER_KEY) return;
    const map = new MapLibreMap({
      container: containerRef.current!,
      style: theme === "dark" ? DARK_STYLE : LIGHT_STYLE,
      zoom: 10,
      minZoom: 5,
      maxZoom: 21,
    });
    mapRef.current = map;

    const pinLayers = ["pins-base", "pins-selected"];
    let initialized = false;
    let active = true;

    map.on("style.load", () => {
      void ensurePinLayers(map, () => active)
        .then(() => {
          if (!active) return;
          // Re-runs the pin-data and highlight-filter effects against the
          // freshly loaded style.
          setStyleVersion((v) => v + 1);
        })
        .catch((error: unknown) => {
          console.error("Failed to load map pin icons", error);
        });
    });

    map.on("load", () => {
      if (initialized) return;
      initialized = true;

        // Typed overload: MapLibre v6 accepts string[] layer ids directly.
        map.on("click", pinLayers, (e) => {
          const id = e.features?.[0]?.properties?.id;
          if (!id) return;

          const pinId = String(id);
          onSelectRef.current(pinId);
          fitPinNeighborhood(map, pinsRef.current, pinId);
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

        // Clicking a cluster zooms in until its pins spread apart.
        map.on("click", "pins-cluster", (e) => {
          const clusterId = e.features?.[0]?.properties?.cluster_id;
          if (typeof clusterId !== "number") return;
          const source = map.getSource("pins") as GeoJSONSource | undefined;
          if (!source) return;
          void source.getClusterExpansionZoom(clusterId).then((zoom) => {
            if (e.lngLat) map.easeTo({ center: e.lngLat, zoom });
          });
        });
        map.on("mouseenter", "pins-cluster", () => {
          map.getCanvas().style.cursor = "pointer";
        });
        map.on("mousemove", "pins-cluster", (e) => {
          const count = e.features?.[0]?.properties?.point_count;
          if (typeof count === "number") {
            setClusterHover({ count, x: e.point.x, y: e.point.y });
          }
        });
        map.on("mouseleave", "pins-cluster", () => {
          map.getCanvas().style.cursor = "";
          setClusterHover(null);
        });

        map.on("click", (e) => {
          const hit = map.queryRenderedFeatures(e.point, {
            layers: [...pinLayers, "pins-cluster"],
          });
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
      console.log("zoom:", map.getZoom());
      const [south, west, north, east] = boundsToValidBbox(map.getBounds());
      onBoundsRef.current(`${south},${west},${north},${east}`, {
        lat: map.getCenter().lat,
        lng: map.getCenter().lng,
      });
    });

    return () => {
      active = false;
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
          category_id: p.category_id,
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

    // Keep the base layer populated so a selected pin's nearby neighbors
    // remain visible; the selected layer provides the highlight overlay.
    const baseFilter: ExpressionSpecification = notCluster;
    const selectedFilter: ExpressionSpecification = highlightId
      ? ["all", notCluster, ["==", ["get", "id"], highlightId]]
      : notCluster;
    map.setFilter("pins-base", baseFilter);
    map.setFilter("pins-selected", selectedFilter);
  }, [highlightId, styleReady, styleVersion]);

  if (!MAPTILER_KEY) {
    return (
      <div className="relative h-full w-full overflow-hidden">
        <div className="flex h-full w-full items-center justify-center bg-zinc-100 p-6 dark:bg-zinc-900">
          <div
            role="status"
            className="max-w-sm rounded-2xl bg-white/95 p-6 text-center shadow-lg ring-1 ring-zinc-200 backdrop-blur dark:bg-zinc-900/95 dark:ring-zinc-700"
          >
            <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
              Map unavailable
            </p>
            <p className="mt-2 text-sm leading-relaxed text-zinc-500 dark:text-zinc-400">
              The map can&apos;t load because the MapTiler API key isn&apos;t
              configured for this environment. Set{" "}
              <code className="rounded bg-zinc-100 px-1 py-0.5 text-xs text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                NEXT_PUBLIC_MAPTILER_API_KEY
              </code>{" "}
              and restart the dev server.
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="relative h-full w-full">
      <div ref={containerRef} className="h-full w-full" />
      {/* Canvas-drawn markers aren't focusable, so expose each pin as a
          visually hidden button that appears over the map when focused —
          the keyboard/screen-reader route to every pin. */}
      <ul
        aria-label="Pins on the map"
        className="absolute left-0 top-0 z-10 m-0 list-none p-0"
      >
        {pins.map((pin) => (
          <li key={pin.id} className="contents">
            <button
              type="button"
              onClick={() => onSelectPin(pin.id)}
              className="sr-only left-3 top-3 focus-visible:[clip-path:none] focus-visible:h-auto focus-visible:w-auto focus-visible:m-0 focus-visible:overflow-visible focus-visible:whitespace-normal focus-visible:rounded-full focus-visible:bg-white/95 focus-visible:px-4 focus-visible:py-2 focus-visible:text-sm focus-visible:font-medium focus-visible:text-zinc-900 focus-visible:shadow-lg focus-visible:ring-1 focus-visible:ring-zinc-200 dark:focus-visible:bg-zinc-900/95 dark:focus-visible:text-zinc-100 dark:focus-visible:ring-zinc-700"
            >
              {pin.caption?.trim() || "Untitled pin"}
              {pin.username ? ` — by ${pin.username}` : ""}
            </button>
          </li>
        ))}
      </ul>
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
      {clusterHover && (
        <div
          className="pointer-events-none absolute z-20 -translate-x-1/2 -translate-y-[calc(100%+8px)]"
          style={{ left: clusterHover.x, top: clusterHover.y }}
        >
          <div className="rounded bg-zinc-900/95 px-2.5 py-1 text-xs text-white shadow-lg">
            Click to expand {clusterHover.count} pins
          </div>
        </div>
      )}
    </div>
  );
}
