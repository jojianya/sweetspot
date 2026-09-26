import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { IoPinSharp } from "react-icons/io5";
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

const PIN_ICON_SIZE = 54; //64
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

function drawCategoryGlyph(ctx: CanvasRenderingContext2D, categoryId: number): void {
  ctx.save();
  ctx.strokeStyle = "#ffffff";
  ctx.fillStyle = "#ffffff";
  ctx.lineWidth = 8;
  ctx.lineCap = "round";
  ctx.lineJoin = "round";

  switch (categoryId) {
    case 1: // Food
      ctx.beginPath();
      ctx.moveTo(96, 61);
      ctx.lineTo(96, 77);
      ctx.moveTo(86, 61);
      ctx.lineTo(86, 74);
      ctx.lineTo(106, 74);
      ctx.lineTo(106, 61);
      ctx.moveTo(96, 77);
      ctx.lineTo(96, 115);
      ctx.moveTo(148, 61);
      ctx.bezierCurveTo(138, 73, 138, 84, 138, 88);
      ctx.lineTo(150, 88);
      ctx.lineTo(150, 115);
      ctx.stroke();
      break;
    case 2: // Nature
      ctx.beginPath();
      ctx.moveTo(128, 113);
      ctx.lineTo(128, 76);
      ctx.moveTo(128, 96);
      ctx.bezierCurveTo(111, 95, 106, 84, 108, 72);
      ctx.bezierCurveTo(120, 72, 128, 79, 128, 91);
      ctx.moveTo(128, 86);
      ctx.bezierCurveTo(130, 70, 141, 64, 153, 66);
      ctx.bezierCurveTo(153, 79, 144, 88, 128, 91);
      ctx.stroke();
      break;
    case 3: // Event
      ctx.strokeRect(98, 69, 60, 45);
      ctx.beginPath();
      ctx.moveTo(98, 83);
      ctx.lineTo(158, 83);
      ctx.moveTo(112, 62);
      ctx.lineTo(112, 75);
      ctx.moveTo(144, 62);
      ctx.lineTo(144, 75);
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(113, 97, 3, 0, Math.PI * 2);
      ctx.arc(128, 97, 3, 0, Math.PI * 2);
      ctx.arc(143, 97, 3, 0, Math.PI * 2);
      ctx.fill();
      break;
    case 4: // Nightlife
      ctx.beginPath();
      ctx.moveTo(94, 63);
      ctx.lineTo(162, 63);
      ctx.lineTo(132, 91);
      ctx.lineTo(132, 111);
      ctx.moveTo(117, 114);
      ctx.lineTo(147, 114);
      ctx.stroke();
      break;
    case 5: // Art
      ctx.strokeRect(98, 67, 60, 48);
      ctx.beginPath();
      ctx.arc(143, 80, 5, 0, Math.PI * 2);
      ctx.moveTo(105, 105);
      ctx.lineTo(119, 90);
      ctx.lineTo(129, 100);
      ctx.lineTo(138, 91);
      ctx.lineTo(152, 106);
      ctx.stroke();
      break;
    case 6: // Sports
      ctx.beginPath();
      ctx.arc(128, 88, 27, 0, Math.PI * 2);
      ctx.moveTo(119, 73);
      ctx.lineTo(137, 73);
      ctx.lineTo(143, 89);
      ctx.lineTo(128, 100);
      ctx.lineTo(113, 89);
      ctx.closePath();
      ctx.moveTo(119, 73);
      ctx.lineTo(111, 64);
      ctx.moveTo(137, 73);
      ctx.lineTo(145, 64);
      ctx.moveTo(113, 89);
      ctx.lineTo(101, 91);
      ctx.moveTo(143, 89);
      ctx.lineTo(155, 91);
      ctx.moveTo(128, 100);
      ctx.lineTo(128, 115);
      ctx.stroke();
      break;
    case 7: // Travel
      ctx.strokeRect(99, 73, 58, 41);
      ctx.beginPath();
      ctx.moveTo(114, 73);
      ctx.lineTo(114, 64);
      ctx.lineTo(142, 64);
      ctx.lineTo(142, 73);
      ctx.moveTo(99, 86);
      ctx.lineTo(157, 86);
      ctx.moveTo(128, 86);
      ctx.lineTo(128, 98);
      ctx.stroke();
      break;
    default: // Other
      ctx.beginPath();
      ctx.moveTo(104, 65);
      ctx.lineTo(148, 65);
      ctx.lineTo(158, 75);
      ctx.lineTo(158, 105);
      ctx.lineTo(104, 105);
      ctx.closePath();
      ctx.moveTo(148, 65);
      ctx.lineTo(148, 76);
      ctx.lineTo(158, 76);
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(119, 85, 4, 0, Math.PI * 2);
      ctx.fill();
  }

  ctx.restore();
}

function makePinIcon(
  size: number,
  color: string,
  ringColor: string | null,
  categoryId: number
): Promise<ImageData> {
  const svg = renderToStaticMarkup(
    createElement(IoPinSharp, {
      color,
      size: PIN_VIEWBOX_SIZE,
      stroke: ringColor ?? color,
      strokeWidth: ringColor ? 12 : 0,
      strokeLinejoin: "round",
    })
  ).replaceAll("currentColor", color);
  const imageURL = URL.createObjectURL(
    new Blob([svg], { type: "image/svg+xml;charset=utf-8" })
  );

  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => {
      URL.revokeObjectURL(imageURL);
      const canvas = document.createElement("canvas");
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        reject(new Error("Unable to create a 2D canvas for map pin icons"));
        return;
      }

      ctx.drawImage(image, 0, 0, size, size);
      ctx.save();
      ctx.scale(size / PIN_VIEWBOX_SIZE, size / PIN_VIEWBOX_SIZE);
      ctx.translate(0, -42);
      drawCategoryGlyph(ctx, categoryId);
      ctx.restore();
      resolve(ctx.getImageData(0, 0, size, size));
    };
    image.onerror = () => {
      URL.revokeObjectURL(imageURL);
      reject(new Error("Unable to render the IoPinSharp map icon"));
    };
    image.src = imageURL;
  });
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
export async function ensurePinLayers(
  map: MaplibreMap,
  isActive: () => boolean = () => true
): Promise<void> {
  const images = await Promise.all([
    ...CATEGORY_IDS.flatMap((categoryId) => [
      makePinIcon(
        PIN_ICON_SIZE,
        colorForCategory(categoryId, false),
        null,
        categoryId
      ).then((image) => [`pin-cat-${categoryId}`, image] as const),
      makePinIcon(
        PIN_ICON_SIZE,
        colorForCategory(categoryId, true),
        "#ffffff",
        categoryId
      ).then((image) => [`pin-cat-${categoryId}-selected`, image] as const),
    ]),
    makePinIcon(PIN_ICON_SIZE, DOT_COLORS[0], null, 8).then(
      (image) => ["pin-default", image] as const
    ),
    makePinIcon(PIN_ICON_SIZE, DOT_COLORS_SELECTED[0], "#ffffff", 8).then(
      (image) => ["pin-selected", image] as const
    ),
  ]);
  if (!isActive()) return;
  for (const [id, image] of images) registerImage(map, id, image);

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
        "icon-allow-overlap": true,
      },
    });
  } else {
    // A hot reload/style diff can retain a layer while replacing its images.
    map.setLayoutProperty("pins-base", "icon-image", categoryTintMatch);
    map.setLayoutProperty("pins-base", "icon-size", 0.75);
    map.setLayoutProperty("pins-base", "icon-anchor", "bottom");
    map.setLayoutProperty("pins-base", "icon-allow-overlap", true);
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
        "icon-allow-overlap": true,
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
    map.setLayoutProperty("pins-selected", "icon-allow-overlap", true);
  }
}
