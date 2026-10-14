import { describe, expect, it } from "vitest";
import {
  buildCreatePinPayload,
  MAX_PIN_PHOTOS,
  MAX_PIN_PHOTO_SIZE,
  validatePhotoFiles,
} from "./photoValidation";

function file(name: string, type: string, size: number): File {
  const blob = new Blob([new Uint8Array(size)], { type });
  return new File([blob], name, { type });
}

describe("validatePhotoFiles", () => {
  it("accepts JPG and PNG within count and size", () => {
    expect(validatePhotoFiles([file("a.jpg", "image/jpeg", 100)])).toBeNull();
    expect(validatePhotoFiles([file("a.png", "image/png", MAX_PIN_PHOTO_SIZE)])).toBeNull();
  });

  it("rejects non-JPG/PNG types first", () => {
    expect(validatePhotoFiles([file("a.gif", "image/gif", 100)])).toBe(
      "Only JPG and PNG images are allowed"
    );
    expect(
      validatePhotoFiles([
        file("a.jpg", "image/jpeg", 100),
        file("b.webp", "image/webp", 100),
      ])
    ).toBe("Only JPG and PNG images are allowed");
  });

  it("rejects more than the photo cap", () => {
    const picked = Array.from({ length: MAX_PIN_PHOTOS + 1 }, (_, i) =>
      file(`${i}.jpg`, "image/jpeg", 100)
    );
    expect(validatePhotoFiles(picked)).toBe(
      `You can upload at most ${MAX_PIN_PHOTOS} photos`
    );
  });

  it("rejects oversized photos", () => {
    expect(validatePhotoFiles([file("big.jpg", "image/jpeg", MAX_PIN_PHOTO_SIZE + 1)])).toBe(
      "Each photo must be 10MB or smaller"
    );
  });

  it("accepts an empty selection (no-op, caller keeps prior files)", () => {
    expect(validatePhotoFiles([])).toBeNull();
  });
});

describe("buildCreatePinPayload", () => {
  const photo = file("a.jpg", "image/jpeg", 100);

  it("requires at least one photo", () => {
    expect(
      buildCreatePinPayload({ lat: 1, lng: 2, categoryId: 3, caption: "hi", files: [] })
    ).toEqual({ error: "Select at least one photo" });
  });

  it("trims the caption and nulls a blank one", () => {
    expect(
      buildCreatePinPayload({ lat: 1, lng: 2, categoryId: 3, caption: "  hi  ", files: [photo] })
    ).toEqual({
      payload: { lat: 1, lng: 2, categoryId: 3, caption: "hi", photos: [photo] },
    });
    expect(
      buildCreatePinPayload({ lat: 1, lng: 2, categoryId: 3, caption: "   ", files: [photo] })
    ).toEqual({
      payload: { lat: 1, lng: 2, categoryId: 3, caption: null, photos: [photo] },
    });
  });
});
