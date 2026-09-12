"use client";

import { useEffect, useRef } from "react";
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

const MAP_STYLE = `https://api.maptiler.com/maps/toner-lite/style.json?key=${process.env.NEXT_PUBLIC_MAPTILER_API_KEY}`;

setWorkerUrl("/maplibre-gl-worker.js");

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
  const userMarkerRef = useRef<MaplibreMarker | null>(null);
  const { position } = useGeolocation();

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
      // center: [78.4867, 17.385],
      zoom: 10,
      minZoom: 5, // don't let users zoom out past city level
      maxZoom: 18,
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
