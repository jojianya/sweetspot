import { type ExpressionSpecification, type Map as MaplibreMap } from "maplibre-gl";

export type GeoFeature = {
  type: "Feature";
  geometry: { type: "Point"; coordinates: [number, number] };
  properties: {
    id: string;
    caption: string;
    username: string;
    cover: string;
  };
};

type GeoJSONLike = {
  type: "FeatureCollection";
  features: GeoFeature[];
};

export const EMPTY_GEOJSON: GeoJSONLike = {
  type: "FeatureCollection",
  features: [],
};

// Pins stop clustering once the map zooms past this level.
export const CLUSTER_MAX_ZOOM = 13;

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

// Filters that keep cluster features out of the per-pin symbol layers.
export const notCluster: ExpressionSpecification = ["!", ["has", "point_count"]];

// setStyle() (theme swap) drops runtime-added sources and layers and
// fires "style.load" again ("load" only fires once per map), but the
// diff path can keep previously added images, so re-creation must be
// guarded per resource.
export function ensurePinLayers(map: MaplibreMap): void {
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
    map.addSource("pins", {
      type: "geojson",
      data: EMPTY_GEOJSON,
      cluster: true,
      clusterRadius: 50,
      clusterMaxZoom: CLUSTER_MAX_ZOOM,
    });
  }

  if (!map.getLayer("pins-cluster")) {
    map.addLayer({
      id: "pins-cluster",
      type: "circle",
      source: "pins",
      filter: ["has", "point_count"],
      paint: {
        "circle-color": [
          "step",
          ["get", "point_count"],
          "#fb7185",
          10,
          "#e11d48",
          100,
          "#9f1239",
        ],
        "circle-radius": ["step", ["get", "point_count"], 16, 10, 21, 100, 27],
        "circle-stroke-width": 2,
        "circle-stroke-color": "#ffffff",
      },
    });
  }

  if (!map.getLayer("pins-cluster-label")) {
    map.addLayer({
      id: "pins-cluster-label",
      type: "symbol",
      source: "pins",
      filter: ["has", "point_count"],
      layout: {
        "text-field": ["get", "point_count"],
        "text-size": 12,
        "text-font": ["Noto Sans Bold"],
      },
      paint: {
        "text-color": "#ffffff",
      },
    });
  }

  if (!map.getLayer("pins-base")) {
    map.addLayer({
      id: "pins-base",
      type: "symbol",
      source: "pins",
      filter: notCluster,
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
      filter: notCluster,
      layout: {
        "icon-image": "pin-selected",
        "icon-size": 1.05,
        "icon-anchor": "bottom",
      },
    });
  }
}