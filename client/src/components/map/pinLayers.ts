import {
  type ExpressionSpecification,
  type GeoJSONSource,
  type Map as MaplibreMap,
} from "maplibre-gl";

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
export const CLUSTER_MAX_ZOOM = 8;

// Set to false to disable clustering without changing the source or layer setup.
export const CLUSTERING_ENABLED = true;

const PIN_ICON_SIZE = 64;
const PIN_VIEWBOX_SIZE = 256;

// These are the same category colors used by CategoryDropdown. The map
// creates color variants, but every variant uses the exact same pin shape.
const DOT_COLORS = [
  "#e11d48", // 0 - rose/red
  "#f97316", // 1 - orange
  "#eab308", // 2 - yellow/amber
  "#22c55e", // 3 - green
  "#0ea5e9", // 4 - blue
  "#8b5cf6", // 5 - purple
];

// Darker variants keep the selected marker legible over the map.
const DOT_COLORS_SELECTED = [
  "#9f1239", // rose-800
  "#c2410c", // orange-700
  "#a16207", // yellow-700
  "#15803d", // green-700
  "#0369a1", // blue-700
  "#6d28d9", // purple-700
];

const CATEGORY_IDS = [1, 2, 3, 4, 5, 6, 7, 8];

function colorForCategory(categoryId: number, selected: boolean): string {
  const colors = selected ? DOT_COLORS_SELECTED : DOT_COLORS;
  return colors[categoryId % colors.length];
}

/**
 * Draws the geometry of react-icons/pi's PiMapPinSimpleLight using canvas
 * primitives so MapLibre receives a raster ImageData rather than a React
 * component. The source icon uses a 256x256 viewBox with a 54px outer
 * circle, a 42px circular opening, and a centered 12px stem.
 *
 * Keeping the geometry as primitives avoids relying on Path2D parsing for
 * runtime-generated map images while preserving the canonical silhouette.
 */
function drawSimplePin(
  ctx: CanvasRenderingContext2D,
  size: number,
  color: string,
  ringColor: string | null
): void {
  const scale = size / PIN_VIEWBOX_SIZE;
  const centerX = 128;
  const headY = 72;
  const outerRadius = 54;
  const innerRadius = 42;
  const stemHalfWidth = 6;
  const stemTopY = 125.66;
  const stemBottomY = 232;
  const stemBottomRadius = 6;
  const ringWidth = 6;

  ctx.save();
  ctx.translate(
    (size - PIN_VIEWBOX_SIZE * scale) / 2,
    (size - PIN_VIEWBOX_SIZE * scale) / 2
  );
  ctx.scale(scale, scale);

  if (ringColor) {
    // Draw the selected halo first. The colored body is painted over its
    // inner half, leaving a clean white outline around the outside.
    ctx.save();
    ctx.strokeStyle = ringColor;
    ctx.lineWidth = ringWidth;
    ctx.lineCap = "round";
    ctx.lineJoin = "round";

    ctx.beginPath();
    ctx.arc(centerX, headY, outerRadius, 0, Math.PI * 2);
    ctx.stroke();

    ctx.beginPath();
    ctx.moveTo(centerX - stemHalfWidth, stemTopY);
    ctx.lineTo(centerX - stemHalfWidth, stemBottomY);
    ctx.arc(
      centerX,
      stemBottomY,
      stemBottomRadius,
      Math.PI,
      0,
      true
    );
    ctx.lineTo(centerX + stemHalfWidth, stemTopY);
    ctx.stroke();
    ctx.restore();
  }

  // Colored outer ring.
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(centerX, headY, outerRadius, 0, Math.PI * 2);
  ctx.fill();

  // Centered stem with the rounded bottom from PiMapPinSimpleLight.
  ctx.beginPath();
  ctx.moveTo(centerX - stemHalfWidth, stemTopY);
  ctx.lineTo(centerX - stemHalfWidth, stemBottomY);
  ctx.arc(
    centerX,
    stemBottomY,
    stemBottomRadius,
    Math.PI,
    0,
    true
  );
  ctx.lineTo(centerX + stemHalfWidth, stemTopY);
  ctx.closePath();
  ctx.fill();

  // The opening is transparent, matching the SVG icon's compound path.
  ctx.save();
  ctx.globalCompositeOperation = "destination-out";
  ctx.beginPath();
  ctx.arc(centerX, headY, innerRadius, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();

  ctx.restore();
}

function makePinIcon(
  size: number,
  color: string,
  ringColor: string | null
): ImageData {
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    throw new Error("Unable to create a 2D canvas for map pin icons");
  }

  drawSimplePin(ctx, size, color, ringColor);
  return ctx.getImageData(0, 0, size, size);
}

function registerImage(
  map: MaplibreMap,
  id: string,
  image: ImageData
): void {
  // Updating an existing image also repairs a hot-reloaded/stale image from
  // an earlier marker implementation without changing layer IDs.
  if (map.hasImage(id)) {
    map.updateImage(id, image);
  } else {
    map.addImage(id, image);
  }
}

// Filters that keep cluster features out of the per-pin symbol layers.
export const notCluster: ExpressionSpecification = ["!", ["has", "point_count"]];

// setStyle() (theme swap) drops runtime-added sources and layers and
// fires "style.load" again ("load" only fires once per map), but the
// diff path can keep previously added images, so re-creation must be
// guarded per resource.
export function ensurePinLayers(map: MaplibreMap): void {
  // Category images differ only by tint. There are no category glyphs or
  // category-specific shapes in the marker set.
  for (const categoryId of CATEGORY_IDS) {
    registerImage(
      map,
      `pin-cat-${categoryId}`,
      makePinIcon(
        PIN_ICON_SIZE,
        colorForCategory(categoryId, false),
        null
      )
    );
    registerImage(
      map,
      `pin-cat-${categoryId}-selected`,
      makePinIcon(
        PIN_ICON_SIZE,
        colorForCategory(categoryId, true),
        "#ffffff"
      )
    );
  }

  // Fallback images for features without a recognized category_id.
  registerImage(
    map,
    "pin-default",
    makePinIcon(PIN_ICON_SIZE, DOT_COLORS[0], null)
  );
  registerImage(
    map,
    "pin-selected",
    makePinIcon(PIN_ICON_SIZE, DOT_COLORS_SELECTED[0], "#ffffff")
  );

  const clusterOptions = {
    cluster: CLUSTERING_ENABLED,
    clusterRadius: 35,
    clusterMaxZoom: CLUSTER_MAX_ZOOM,
  };

  const pinSource = map.getSource("pins") as GeoJSONSource | undefined;
  if (!pinSource) {
    map.addSource("pins", {
      type: "geojson",
      data: EMPTY_GEOJSON,
      ...clusterOptions,
    });
  } else {
    // Also update an existing source so the flag takes effect after HMR or
    // a style reload without removing the cluster layers or expressions.
    void pinSource.setClusterOptions(clusterOptions);
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

  // The match expression selects a color tint, not a different icon shape.
  const categoryTintMatch: ExpressionSpecification = [
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
    "pin-default",
  ];

  const selectedCategoryTintMatch: ExpressionSpecification = [
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
    "pin-selected",
  ];

  if (!map.getLayer("pins-base")) {
    map.addLayer({
      id: "pins-base",
      type: "symbol",
      source: "pins",
      filter: notCluster,
      layout: {
        "icon-image": categoryTintMatch,
        "icon-size": 0.75,
        "icon-anchor": "bottom",
        "icon-allow-overlap": false,
      },
    });
  } else {
    // A hot reload/style diff can retain a layer while replacing its images.
    map.setLayoutProperty("pins-base", "icon-image", categoryTintMatch);
    map.setLayoutProperty("pins-base", "icon-size", 0.75);
    map.setLayoutProperty("pins-base", "icon-anchor", "bottom");
    map.setLayoutProperty("pins-base", "icon-allow-overlap", false);
  }

  if (!map.getLayer("pins-selected")) {
    map.addLayer({
      id: "pins-selected",
      type: "symbol",
      source: "pins",
      filter: notCluster,
      layout: {
        "icon-image": selectedCategoryTintMatch,
        "icon-size": 1.05,
        "icon-anchor": "bottom",
        "icon-allow-overlap": false,
      },
    });
  } else {
    map.setLayoutProperty(
      "pins-selected",
      "icon-image",
      selectedCategoryTintMatch
    );
    map.setLayoutProperty("pins-selected", "icon-size", 1.05);
    map.setLayoutProperty("pins-selected", "icon-anchor", "bottom");
    map.setLayoutProperty("pins-selected", "icon-allow-overlap", false);
  }
}
