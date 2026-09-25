import { type ExpressionSpecification, type Map as MaplibreMap } from "maplibre-gl";

export type GeoFeature = {
  type: "Feature";
  geometry: { type: "Point"; coordinates: [number, number] };
  properties: {
    id: string;
    caption: string;
    username: string;
    cover: string;
    category_id: number;
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

// Uniform pin color (rose) for all pins
const PIN_COLOR = "#e11d48";
const PIN_COLOR_SELECTED = "#9f1239";

// Draw a phosphor-style pin using canvas primitives (circle head + stem + tip + inner dot)
// Uniform color for all pins (rose)
function drawPhosphorPin(
  ctx: CanvasRenderingContext2D,
  size: number,
  color: string,
  ringColor: string | null
): void {
  ctx.fillStyle = color;

  // Draw pin head (circle)
  ctx.beginPath();
  ctx.arc(size / 2, size * 0.3, size * 0.26, 0, Math.PI * 2);
  ctx.fill();

  // Draw stem (tapered triangle pointing down)
  ctx.beginPath();
  ctx.moveTo(size / 2, size * 0.56);
  ctx.lineTo(size / 2 - size * 0.12, size * 0.8);
  ctx.lineTo(size / 2 + size * 0.12, size * 0.8);
  ctx.closePath();
  ctx.fill();

  // Draw tip (rounded bottom)
  ctx.beginPath();
  ctx.arc(size / 2, size * 0.95, size * 0.08, 0, Math.PI * 2);
  ctx.fill();

  // White ring for selected state
  if (ringColor) {
    ctx.strokeStyle = "#ffffff";
    ctx.lineWidth = size * 0.06;

    // Outer ring around head
    ctx.beginPath();
    ctx.arc(size / 2, size * 0.3, size * 0.31, 0, Math.PI * 2);
    ctx.stroke();

    // Stem outline
    ctx.beginPath();
    ctx.moveTo(size / 2, size * 0.56);
    ctx.lineTo(size / 2 - size * 0.15, size * 0.85);
    ctx.lineTo(size / 2 + size * 0.15, size * 0.85);
    ctx.closePath();
    ctx.stroke();

    // Tip ring
    ctx.beginPath();
    ctx.arc(size / 2, size * 0.95, size * 0.11, 0, Math.PI * 2);
    ctx.stroke();
  }

  // Inner white dot (center of head)
  ctx.fillStyle = "#ffffff";
  ctx.beginPath();
  ctx.arc(size / 2, size * 0.3, size * 0.1, 0, Math.PI * 2);
  ctx.fill();
}

// Uniform pin icon for all pins (rose color)
function makePinIcon(
  size: number,
  color: string,
  ringColor: string | null
): ImageData {
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d")!;
  drawPhosphorPin(ctx, size, color, ringColor);
  return ctx.getImageData(0, 0, size, size);
}

// Filters that keep cluster features out of the per-pin symbol layers.
export const notCluster: ExpressionSpecification = ["!", ["has", "point_count"]];

// setStyle() (theme swap) drops runtime-added sources and layers and
// fires "style.load" again ("load" only fires once per map), but the
// diff path can keep previously added images, so re-creation must be
// guarded per resource.
export function ensurePinLayers(map: MaplibreMap): void {
  // Single uniform pin images for all pins (rose color)
  if (!map.hasImage("pin-default")) {
    map.addImage("pin-default", makePinIcon(64, PIN_COLOR, null));
  }
  if (!map.hasImage("pin-selected")) {
    map.addImage(
      "pin-selected",
      makePinIcon(64, PIN_COLOR_SELECTED, "#ffffff")
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

  // Single uniform pin layers for all pins
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