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

// Category colors from CategoryDropdown (DOT_COLORS)
const DOT_COLORS = [
  "#e11d48", // 0 - rose/red
  "#f97316", // 1 - orange
  "#eab308", // 2 - yellow/amber
  "#22c55e", // 3 - green
  "#0ea5e9", // 4 - blue
  "#8b5cf6", // 5 - purple
];

// Darker variants for selected state (reduce lightness ~15%)
const DOT_COLORS_SELECTED = [
  "#9f1239", // rose-800
  "#c2410c", // orange-700
  "#a16207", // yellow-700
  "#15803d", // green-700
  "#0369a1", // blue-700
  "#6d28d9", // purple-700
];

// Map category_id (1-8) to DOT_COLORS index: category_id % 6
function colorForCategory(categoryId: number): string {
  return DOT_COLORS[categoryId % DOT_COLORS.length];
}

function colorForCategorySelected(categoryId: number): string {
  return DOT_COLORS_SELECTED[categoryId % DOT_COLORS_SELECTED.length];
}

// Category icons drawn on canvas (matching icons.tsx stroke style)
function drawCategoryIcon(
  ctx: CanvasRenderingContext2D,
  cx: number,
  cy: number,
  size: number,
  categoryId: number
): void {
  const s = size * 0.35; // icon scale factor
  const r = size * 0.26; // circle radius (matches makePinIcon)
  const stroke = size * 0.035;
  ctx.strokeStyle = "#ffffff";
  ctx.lineWidth = stroke;
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.fillStyle = "none";

  switch (categoryId) {
    case 1: // Food — Utensils (fork & knife)
      ctx.beginPath();
      ctx.moveTo(cx - s * 0.8, cy - s * 0.3);
      ctx.lineTo(cx + s * 0.8, cy + s * 0.9);
      ctx.moveTo(cx + s * 0.8, cy - s * 0.3);
      ctx.lineTo(cx - s * 0.8, cy + s * 0.9);
      ctx.stroke();
      break;
    case 2: // Nature — Tree
      // Crown: two arcs on the circle boundary (matching SVG TreeIcon)
      // Circle: center (cx, cy), radius r = size * 0.26
      // Crown arcs at circle boundary: center offset by r * 0.5, radius r * 0.5
      const crownR = r * 0.5;
      const crownOffsetX = r * 0.5;
      // Left crown arc (center-left)
      ctx.beginPath();
      ctx.arc(cx - crownOffsetX, cy, crownR, Math.PI, 0);
      // Right crown arc (center-right)
      ctx.moveTo(cx + crownOffsetX + crownR, cy);
      ctx.arc(cx + crownOffsetX, cy, crownR, Math.PI, 0);
      // Trunk: from circle bottom (cy + r) down to tip area
      ctx.moveTo(cx, cy + r);
      ctx.lineTo(cx, cy + r + s * 0.5);
      ctx.stroke();
      break;
    case 3: // Event — Calendar
      const calW = s * 1.1;
      const calH = s * 1.1;
      ctx.strokeRect(cx - calW / 2, cy - calH / 2, calW, calH);
      ctx.beginPath();
      ctx.moveTo(cx - calW / 2 + s * 0.2, cy - calH / 2);
      ctx.lineTo(cx - calW / 2 + s * 0.2, cy - calH / 2 - s * 0.3);
      ctx.moveTo(cx + calW / 2 - s * 0.2, cy - calH / 2);
      ctx.lineTo(cx + calW / 2 - s * 0.2, cy - calH / 2 - s * 0.3);
      ctx.moveTo(cx - calW / 2, cy + s * 0.1);
      ctx.lineTo(cx + calW / 2, cy + s * 0.1);
      ctx.stroke();
      break;
    case 4: // Nightlife — Moon
      ctx.beginPath();
      ctx.arc(cx, cy, s * 0.7, -Math.PI / 2, Math.PI / 2, true);
      ctx.bezierCurveTo(
        cx + s * 0.7,
        cy,
        cx + s * 0.7,
        cy + s * 1.4,
        cx,
        cy + s * 0.7
      );
      ctx.stroke();
      break;
    case 5: // Art — Palette
      ctx.beginPath();
      ctx.arc(cx - s * 0.4, cy - s * 0.4, s * 0.35, 0, Math.PI * 2);
      ctx.arc(cx + s * 0.4, cy - s * 0.1, s * 0.35, 0, Math.PI * 2);
      ctx.arc(cx + s * 0.1, cy + s * 0.5, s * 0.35, 0, Math.PI * 2);
      ctx.arc(cx - s * 0.5, cy + s * 0.2, s * 0.35, 0, Math.PI * 2);
      ctx.stroke();
      // thumb hole
      ctx.beginPath();
      ctx.arc(cx - s * 0.7, cy + s * 0.5, s * 0.15, 0, Math.PI * 2);
      ctx.stroke();
      break;
    case 6: // Sports — Trophy
      ctx.beginPath();
      ctx.moveTo(cx - s * 0.4, cy + s * 0.2);
      ctx.lineTo(cx - s * 0.2, cy - s * 0.5);
      ctx.lineTo(cx, cy + s * 0.2);
      ctx.moveTo(cx + s * 0.4, cy + s * 0.2);
      ctx.lineTo(cx + s * 0.2, cy - s * 0.5);
      ctx.lineTo(cx, cy + s * 0.2);
      ctx.moveTo(cx - s * 0.5, cy + s * 0.8);
      ctx.lineTo(cx + s * 0.5, cy + s * 0.8);
      ctx.moveTo(cx - s * 0.1, cy + s * 0.8);
      ctx.lineTo(cx - s * 0.1, cy + s * 1.2);
      ctx.moveTo(cx + s * 0.1, cy + s * 0.8);
      ctx.lineTo(cx + s * 0.1, cy + s * 1.2);
      ctx.moveTo(cx, cy - s * 0.5);
      ctx.lineTo(cx, cy + s * 0.8);
      ctx.stroke();
      break;
    case 7: // Travel — Globe
      ctx.beginPath();
      ctx.arc(cx, cy, s * 0.7, 0, Math.PI * 2);
      ctx.moveTo(cx - s * 0.7, cy);
      ctx.lineTo(cx + s * 0.7, cy);
      ctx.moveTo(cx, cy - s * 0.7);
      ctx.bezierCurveTo(cx + s * 0.4, cy - s * 0.7, cx + s * 0.4, cy + s * 0.7, cx, cy + s * 0.7);
      ctx.moveTo(cx, cy - s * 0.7);
      ctx.bezierCurveTo(cx - s * 0.4, cy - s * 0.7, cx - s * 0.4, cy + s * 0.7, cx, cy + s * 0.7);
      ctx.stroke();
      break;
    default: // Other (8+) — Pin (default teardrop)
      ctx.beginPath();
      ctx.moveTo(cx, cy - s * 0.5);
      ctx.bezierCurveTo(cx + s * 0.5, cy - s * 0.5, cx + s * 0.5, cy + s * 0.5, cx, cy + s * 0.5);
      ctx.bezierCurveTo(cx - s * 0.5, cy + s * 0.5, cx - s * 0.5, cy - s * 0.5, cx, cy - s * 0.5);
      ctx.stroke();
      break;
  }
}

// Draw a phosphor-style pin using canvas primitives (circle + triangle stem + inner dot)
// This replaces the problematic Path2D approach
function drawPhosphorPin(
  ctx: CanvasRenderingContext2D,
  cx: number,
  cy: number,
  size: number,
  color: string,
  ringColor: string | null,
  dotColor: string
): void {
  // Pin dimensions (phosphor map-pin proportions)
  const r = size * 0.26;           // head circle radius
  const headY = size * 0.3;        // head center Y
  const stemBaseY = size * 0.56;   // where stem meets head
  const tipY = size * 0.95;        // tip of stem

  ctx.fillStyle = color;

  // Draw pin head (circle)
  ctx.beginPath();
  ctx.arc(size / 2, size * 0.3, size * 0.26, 0, Math.PI * 2);
  ctx.fill();

  // Draw stem (tapered triangle pointing down)
  ctx.beginPath();
  ctx.moveTo(size / 2, size * 0.56);           // top center (where stem meets head)
  ctx.lineTo(size / 2 - size * 0.12, size * 0.8);  // left base
  ctx.lineTo(size / 2 + size * 0.12, size * 0.8);  // right base
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

// Base pin icon (legacy fallback, no category)
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
  drawPhosphorPin(ctx, size / 2, size * 0.3, size, color, ringColor, dotColor);
  return ctx.getImageData(0, 0, size, size);
}

// Creates a category-specific pin icon (with embedded category symbol)
function makeCategoryIcon(
  size: number,
  categoryId: number,
  isSelected = false
): ImageData {
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d")!;

  const color = isSelected
    ? colorForCategorySelected(categoryId)
    : colorForCategory(categoryId);
  const ringColor = isSelected ? "#ffffff" : null;

  // Draw phosphor pin base
  drawPhosphorPin(ctx, size / 2, size * 0.3, size, color, ringColor, "#ffffff");

  // Draw category icon in the pin head center
  const headY = size * 0.3;
  const r = size * 0.26;
  drawCategoryIcon(ctx, size / 2, size * 0.3, size, categoryId);

  // White dot overlay (smaller, on top of category icon)
  ctx.fillStyle = "#ffffff";
  ctx.beginPath();
  ctx.arc(size / 2, size * 0.3, size * 0.08, 0, Math.PI * 2);
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
  // Register category-specific pin images (default + selected for each of 8 categories)
  for (let cat = 1; cat <= 8; cat++) {
    const defaultKey = `pin-cat-${cat}`;
    const selectedKey = `pin-cat-${cat}-selected`;

    if (!map.hasImage(defaultKey)) {
      map.addImage(defaultKey, makeCategoryIcon(64, cat, false));
    }
    if (!map.hasImage(selectedKey)) {
      map.addImage(selectedKey, makeCategoryIcon(64, cat, true));
    }
  }

  // Legacy fallback images (kept for any legacy data without category_id)
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

  // Per-category icon mapping for base (unselected) pins
  const categoryIconMatch: ExpressionSpecification = [
    "match",
    ["get", "category_id"],
    1,
    "pin-cat-1",
    2,
    "pin-cat-2",
    3,
    "pin-cat-3",
    4,
    "pin-cat-4",
    5,
    "pin-cat-5",
    6,
    "pin-cat-6",
    7,
    "pin-cat-7",
    8,
    "pin-cat-8",
    "pin-default", // fallback
  ];

  // Per-category icon mapping for selected pins
  const selectedCategoryIconMatch: ExpressionSpecification = [
    "match",
    ["get", "category_id"],
    1,
    "pin-cat-1-selected",
    2,
    "pin-cat-2-selected",
    3,
    "pin-cat-3-selected",
    4,
    "pin-cat-4-selected",
    5,
    "pin-cat-5-selected",
    6,
    "pin-cat-6-selected",
    7,
    "pin-cat-7-selected",
    8,
    "pin-cat-8-selected",
    "pin-selected", // fallback
  ];

  if (!map.getLayer("pins-base")) {
    map.addLayer({
      id: "pins-base",
      type: "symbol",
      source: "pins",
      filter: notCluster,
      layout: {
        "icon-image": categoryIconMatch,
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
        "icon-image": selectedCategoryIconMatch,
        "icon-size": 1.05,
        "icon-anchor": "bottom",
      },
    });
  }
}