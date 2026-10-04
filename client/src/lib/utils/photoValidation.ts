/** Maximum photos per pin and per-photo size cap. Mirrors CreatePinButton. */
export const MAX_PIN_PHOTOS = 5;
export const MAX_PIN_PHOTO_SIZE = 10 * 1024 * 1024;

const ALLOWED_PHOTO_TYPES = /^image\/(jpeg|png)$/;

/**
 * Validates a photo selection for pin creation. Returns the error message to
 * show, or null when the selection is usable. Order matters: MIME first, then
 * count, then size — the first failure wins. Extracted unchanged from
 * CreatePinButton.
 */
export function validatePhotoFiles(picked: File[]): string | null {
  const valid = picked.filter((f) => ALLOWED_PHOTO_TYPES.test(f.type));
  if (valid.length !== picked.length) {
    return "Only JPG and PNG images are allowed";
  }
  if (valid.length > MAX_PIN_PHOTOS) {
    return `You can upload at most ${MAX_PIN_PHOTOS} photos`;
  }
  if (valid.some((f) => f.size > MAX_PIN_PHOTO_SIZE)) {
    return "Each photo must be 10MB or smaller";
  }
  return null;
}

export interface CreatePinPayload {
  lat: number;
  lng: number;
  categoryId: number;
  caption: string | null;
  photos: File[];
}

/**
 * Shapes the create-pin submit payload. Returns an error when no photo is
 * selected; otherwise the trimmed caption (or null) plus the files.
 */
export function buildCreatePinPayload(args: {
  lat: number;
  lng: number;
  categoryId: number;
  caption: string;
  files: File[];
}): { payload: CreatePinPayload } | { error: string } {
  if (args.files.length < 1) {
    return { error: "Select at least one photo" };
  }
  return {
    payload: {
      lat: args.lat,
      lng: args.lng,
      categoryId: args.categoryId,
      caption: args.caption.trim() || null,
      photos: args.files,
    },
  };
}
