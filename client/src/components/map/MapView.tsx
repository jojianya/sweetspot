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

const MAP_STYLE = `https://api.maptiler.com/maps/toner-lite/style.json?key=${process.env.NEXT_PUBLIC_MAPTILER_API_KEY}`;

setWorkerUrl("/maplibre-gl-worker.js");

export interface MapLocation {
  lat: number;
  lng: number;
}

const PENDING_PIN_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" width="36" height="44" viewBox="0 0 36 44" fill="none"><path d="M18 0C8.06 0 0 8.06 0 18c0 2.6.75 5.4 2 7.78C4.6 31 14 44 16.35 41.5L18 40l1.65 1.5C22 44 31.4 31 34 25.78 35.25 23.4 36 20.6 36 18 36 8.06 27.94 0 18 0Z" fill="#e11d48" stroke="#fff" stroke-width="2.5"/><circle cx="18" cy="18" r="6.5" fill="#fff"/></svg>';

interface MapViewProps {
  pins: PinListEntry[];
  flyTo: { lng: number; lat: number } | null;
  pendingLocation: MapLocation | null;
  onBoundsChange: (bbox: string, center: { lat: number; lng: number }) => void;
  onSelectPin: (id: string) => void;
  onSelectLocation: (location: MapLocation) => void;
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
  pendingLocation,
  onBoundsChange,
  onSelectPin,
  onSelectLocation,
}: MapViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MaplibreMap | null>(null);
  const userMarkerRef = useRef<MaplibreMarker | null>(null);
  const pendingMarkerRef = useRef<MaplibreMarker | null>(null);
  const { position } = useGeolocation();
  const [styleReady, setStyleReady] = useState(false);

  const onBoundsRef = useRef(onBoundsChange);
  const onSelectRef = useRef(onSelectPin);
  const onSelectLocationRef = useRef(onSelectLocation);

  useEffect(() => {
    onBoundsRef.current = onBoundsChange;
    onSelectRef.current = onSelectPin;
    onSelectLocationRef.current = onSelectLocation;
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

      map.on("click", (e) => {
        const hit = map.queryRenderedFeatures(e.point, {
          layers: ["pin-circles"],
        });
        if (hit.length > 0) return;
        onSelectLocationRef.current({ lat: e.lngLat.lat, lng: e.lngLat.lng });
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
    if (!pendingLocation) {
      pendingMarkerRef.current?.remove();
      pendingMarkerRef.current = null;
      return;
    }
    const map = mapRef.current;
    if (!map) return;
    if (pendingMarkerRef.current) {
      pendingMarkerRef.current.setLngLat([pendingLocation.lng, pendingLocation.lat]);
      return;
    }
    const el = document.createElement("div");
    el.className = "pointer-events-none";
    el.innerHTML = PENDING_PIN_SVG;
    pendingMarkerRef.current = new Marker({ element: el, anchor: "bottom" })
      .setLngLat([pendingLocation.lng, pendingLocation.lat])
      .addTo(map);
  }, [pendingLocation]);

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
        properties: { id: p.id },
      });
    }

    (source as GeoJSONSource).setData({ type: "FeatureCollection", features });
  }, [pins, styleReady]);

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
