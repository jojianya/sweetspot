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

// Pin post artwork, from the agreed spec:
//
//   pin            28 x 40        (viewBox 0 0 28 40)
//   top circle     28 x 28, centre (14, 14)   -> r=14, widest at y=14
//   white centre   d=8, centre (14, 13)        -> r=4
//   point          tapers from y=20, tip at (14, 40)
//   anchor         (14, 40)
//
// The texture is sized to the pin's own bounding box (tip included) so
// `icon-anchor: "bottom"` puts the geographic point on the very tip rather
// than on transparent padding. Rendered height on the map is therefore
// PIN_IMAGE_HEIGHT * icon-size.
const PIN_IMAGE_WIDTH = 28;
const PIN_IMAGE_HEIGHT = 40;

// Google Maps-style teardrop. 0.85 * 40 = 34px tall on the map; the selected
// variant is 1.0 * 40 = 40px, so both stay in the 32-40px band.
const PIN_BASE_ICON_SIZE = 0.85;
const PIN_SELECTED_ICON_SIZE = 1;

const PIN_IMAGE = "pin-post";
const PIN_IMAGE_SELECTED = "pin-post-selected";

// Image ids from the per-category pin set this design replaced. Kept only so a
// hot-reloaded map can release their textures; nothing references them now.
const STALE_IMAGE_IDS = [
  "pin-default",
  "pin-selected",
  ...[1, 2, 3, 4, 5, 6, 7, 8].flatMap((id) => [`pin-cat-${id}`, `pin-cat-${id}-selected`]),
] as const;

// Google Maps red, plus a slightly darker fill for the selected variant.
const PIN_FILL = "#EA4335";
const PIN_FILL_SELECTED = "#C5221F";

/**
 * Three shapes, unioned by overdraw rather than stitched into one outline:
 *
 *   1. a soft elliptical shadow, centred low and wide enough that its darker
 *      core falls *outside* the narrowing tail, which is what makes it read as
 *      a drop shadow rather than a grey smudge hidden behind an opaque body;
 *   2. the head as a literal <circle> r=14 at (14,14) — the spec's 28x28 top
 *      circle, exact by construction instead of approximated by an arc;
 *   3. the tail, a path running from the tip up each flank.
 *
 * The flanks are tangent-continuous with the circle where they meet it at
 * y=20: radius (12.65, 6) gives tangent (0.429, -0.904), and the control point
 * (24.68, 24.15) sits 4.59px up that line from (26.65, 20). The two flanks
 * mirror about x=14 and meet at (14, 40) with opposing slopes, giving a clean
 * downward point.
 *
 * The tail's own top is the buried chord (4,8)-(24,8). The circle is convex, so
 * that whole segment lies strictly inside the head and is never visible.
 * Nothing is stroked, so there is no seam or stray hairline where the two
 * shapes meet: the flat red plus the shadow is what separates the pin from the
 * basemap, which is also how the Google Maps marker reads at this size.
 */
export function pinSvg(fill: string): string {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 28 40" width="${PIN_IMAGE_WIDTH}" height="${PIN_IMAGE_HEIGHT}"><defs><radialGradient id="pin-shadow" cx="50%" cy="50%" r="50%"><stop offset="0" stop-color="#000" stop-opacity="0.34"/><stop offset="0.62" stop-color="#000" stop-opacity="0.24"/><stop offset="0.85" stop-color="#000" stop-opacity="0.09"/><stop offset="1" stop-color="#000" stop-opacity="0"/></radialGradient></defs><ellipse cx="14" cy="27" rx="13.4" ry="11.5" fill="url(#pin-shadow)"/><circle cx="14" cy="14" r="14" fill="${fill}"/><path d="M4 8L24 8L26.65 20C24.68 24.15 15.8 35.5 14 40C12.2 35.5 3.32 24.15 1.35 20Z" fill="${fill}"/><circle cx="14" cy="13" r="4" fill="#fff"/></svg>`;
}

/**
 * Module-scope cache for the generated pin textures. The artwork is a pure
 * function of the fill colour and never changes at runtime, so each variant
 * is rasterized once and reused. Without this, every style.load (map init and
 * every light/dark toggle) re-decodes the SVG blobs and re-uploads textures.
 */
const iconCache = new Map<string, Promise<ImageData>>();

function makePinIcon(fill: string): Promise<ImageData> {
  const cached = iconCache.get(fill);
  if (cached) return cached;

  const promise = buildPinIcon(fill);
  iconCache.set(fill, promise);
  return promise;
}

function buildPinIcon(fill: string): Promise<ImageData> {
  const imageURL = URL.createObjectURL(
    new Blob([pinSvg(fill)], { type: "image/svg+xml;charset=utf-8" })
  );

  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => {
      URL.revokeObjectURL(imageURL);
      const canvas = document.createElement("canvas");
      canvas.width = PIN_IMAGE_WIDTH;
      canvas.height = PIN_IMAGE_HEIGHT;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        reject(new Error("Unable to create a 2D canvas for map pin icons"));
        return;
      }

      ctx.drawImage(image, 0, 0, PIN_IMAGE_WIDTH, PIN_IMAGE_HEIGHT);
      resolve(ctx.getImageData(0, 0, PIN_IMAGE_WIDTH, PIN_IMAGE_HEIGHT));
    };
    image.onerror = () => {
      URL.revokeObjectURL(imageURL);
      reject(new Error("Unable to render the map pin icon"));
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
    makePinIcon(PIN_FILL).then(
      (image) => [PIN_IMAGE, image] as const
    ),
    makePinIcon(PIN_FILL_SELECTED).then(
      (image) => [PIN_IMAGE_SELECTED, image] as const
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

  if (!map.getLayer("pins-base")) {
    map.addLayer({
      id: "pins-base",
      type: "symbol",
      source: "pins",
      filter: notCluster,
      layout: {
        "icon-image": PIN_IMAGE,
        "icon-size": PIN_BASE_ICON_SIZE,
        "icon-anchor": "bottom",
        "icon-allow-overlap": true,
      },
    });
  } else {
    // A hot reload/style diff can retain a layer while replacing its images.
    map.setLayoutProperty("pins-base", "icon-image", PIN_IMAGE);
    map.setLayoutProperty("pins-base", "icon-size", PIN_BASE_ICON_SIZE);
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
        "icon-image": PIN_IMAGE_SELECTED,
        "icon-size": PIN_SELECTED_ICON_SIZE,
        "icon-anchor": "bottom",
        "icon-allow-overlap": true,
      },
    });
  } else {
    map.setLayoutProperty(
      "pins-selected",
      "icon-image",
      PIN_IMAGE_SELECTED
    );
    map.setLayoutProperty("pins-selected", "icon-size", PIN_SELECTED_ICON_SIZE);
    map.setLayoutProperty("pins-selected", "icon-anchor", "bottom");
    map.setLayoutProperty("pins-selected", "icon-allow-overlap", true);
  }

  // The per-category icons this design replaced are still registered on a map
  // that hot-reloaded (or kept its images through a setStyle diff). Drop them
  // only after both symbol layers point at the new images, since MapLibre
  // refuses to remove an image a layer still references.
  for (const id of STALE_IMAGE_IDS) {
    if (map.hasImage(id)) map.removeImage(id);
  }
}
