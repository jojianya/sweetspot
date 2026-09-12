"use client";

import { useEffect, useRef } from "react";
import {
  Map as MapLibreMap,
  setWorkerUrl,
  type GeoJSONSource,
  type LngLatBounds,
  type Map as MaplibreMap,
} from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import type { PinListEntry } from "@/lib/types";
import { parsePoint } from "@/lib/format";

const MAP_STYLE = `https://api.maptiler.com/maps/toner-lite/style.json?key=${process.env.NEXT_PUBLIC_MAPTILER_API_KEY}`;

setWorkerUrl("/maplibre-gl-worker.js");

function boundsToValidBbox(b: LngLatBounds): [number, number, number, number] {
  const south = Math.max(-90, Math.min(90, b.getSouth()));
  const north = Math.max(-90, Math.min(90, b.getNorth()));
  const west = b.getWest();
  const east = b.getEast();
  if (east - west >= 360) return [south, -180, north, 180];
  const wrap = (lng: number) => (((lng % 360) + 540) % 360) - 180;
  const lngMin = wrap(west);
  const lngMax = wrap(east);
  if (lngMax < lngMin) return [south, -180, north, 180];
  return [south, lngMin, north, lngMax];
}

interface MapViewProps {
  pins: PinListEntry[];
  flyTo: { lng: number; lat: number } | null;
  onBoundsChange: (bbox: string, center: { lat: number; lng: number }) => void;
  onSelectPin: (id: string) => void;
}

interface GeoFeature {
  type: "Feature";
  geometry: { type: "Point"; coordinates: [number, number] };
  properties: { id: string };
}

const EMPTY_GEOJSON: GeoJSONLike = {
  type: "FeatureCollection",
  features: [],
};

type GeoJSONLike = {
  type: "FeatureCollection";
  features: GeoFeature[];
};

export default function MapView({
  pins,
  flyTo,
  onBoundsChange,
  onSelectPin,
}: MapViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MaplibreMap | null>(null);

  const onBoundsRef = useRef(onBoundsChange);
  const onSelectRef = useRef(onSelectPin);

  useEffect(() => {
    onBoundsRef.current = onBoundsChange;
    onSelectRef.current = onSelectPin;
  });

  useEffect(() => {
    const map = new MapLibreMap({
      container: containerRef.current!,
      style: MAP_STYLE,
      center: [78.4867, 17.385],
      zoom: 10,
    });
    mapRef.current = map;

    map.on("load", () => {
      map.addSource("pins", { type: "geojson", data: EMPTY_GEOJSON });
      map.addLayer({
        id: "pin-circles",
        type: "circle",
        source: "pins",
        paint: {
          "circle-radius": 9,
          "circle-color": "#e11d48",
          "circle-stroke-width": 2.5,
          "circle-stroke-color": "#ffffff",
        },
      });

      map.on("click", "pin-circles", (e) => {
        const feature = e.features?.[0];
        const id = feature?.properties?.id;
        if (id) onSelectRef.current(String(id));
      });
      map.on("mouseenter", "pin-circles", () => {
        map.getCanvas().style.cursor = "pointer";
      });
      map.on("mouseleave", "pin-circles", () => {
        map.getCanvas().style.cursor = "";
      });

      const [south, west, north, east] = boundsToValidBbox(map.getBounds());
      onBoundsRef.current(`${south},${west},${north},${east}`, {
        lat: map.getCenter().lat,
        lng: map.getCenter().lng,
      });
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
    if (!map || !map.isStyleLoaded()) return;

    const source = map.getSource("pins");
    if (!source) return;

    const features: GeoFeature[] = pins.map((p) => {
      const { lng, lat } = parsePoint(p.location);
      return {
        type: "Feature",
        geometry: { type: "Point", coordinates: [lng, lat] },
        properties: { id: p.id },
      };
    });

    (source as GeoJSONSource).setData({ type: "FeatureCollection", features });
  }, [pins]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !flyTo) return;
    map.flyTo({
      center: [flyTo.lng, flyTo.lat],
      zoom: Math.max(map.getZoom(), 13),
    });
  }, [flyTo]);

  return <div ref={containerRef} className="h-full w-full" />;
}
