"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { CloseIcon } from "@/components/icons";
import { createPin } from "@/lib/api";
import type { Category, CreatedPin, NewPinPhoto } from "@/lib/types";
import type { MapLocation } from "@/components/map/MapView";
import { errorMessage, getCurrentPosition, geolocationAvailable, toGeoCoords } from "@/lib/utils";
import { useAuth } from "@/store/auth";

const MAX_PHOTOS = 5;
const MAX_PHOTO_SIZE = 10 * 1024 * 1024;

interface CreatePinButtonProps {
  lat: number;
  lng: number;
  categories: Category[];
  posting: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (
    pin: CreatedPin,
    photos: NewPinPhoto[]
  ) => void;
  onSetLocation: (location: MapLocation) => void;
}

export default function CreatePinButton({
  lat,
  lng,
  categories,
  posting,
  open,
  onOpenChange,
  onCreated,
  onSetLocation,
}: CreatePinButtonProps) {
  const { token } = useAuth();
  const isLoggedIn = token !== null;
  const [files, setFiles] = useState<File[]>([]);
  const [caption, setCaption] = useState("");
  const [categoryId, setCategoryId] = useState<number>(categories[0]?.id ?? 1);
  const [submitting, setSubmitting] = useState(false);
  const [locating, setLocating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const previews = useMemo(
    () => files.map((f) => URL.createObjectURL(f)),
    [files]
  );

  useEffect(() => {
    return () => {
      previews.forEach((u) => URL.revokeObjectURL(u));
    };
  }, [previews]);

  const handleClose = useCallback(() => {
    onOpenChange(false);
    setFiles([]);
    setCaption("");
    setError(null);
  }, [onOpenChange]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") handleClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, handleClose]);

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handleFiles = (list: FileList | null) => {
    if (!list) return;
    const picked = Array.from(list);
    const valid = picked.filter((f) => /^image\/(jpeg|png)$/.test(f.type));
    if (valid.length !== picked.length) {
      setError("Only JPG and PNG images are allowed");
      return;
    }
    if (valid.length > MAX_PHOTOS) {
      setError(`You can upload at most ${MAX_PHOTOS} photos`);
      return;
    }
    if (valid.some((f) => f.size > MAX_PHOTO_SIZE)) {
      setError("Each photo must be 10MB or smaller");
      return;
    }
    setError(null);
    setFiles(valid);
  };

  const useCurrentLocation = () => {
    if (!geolocationAvailable()) {
      setError("Location is not available in this browser");
      return;
    }
    setLocating(true);
    setError(null);
    getCurrentPosition()
      .then((pos) => {
        setLocating(false);
        onSetLocation(toGeoCoords(pos));
      })
      .catch((e: unknown) => {
        setLocating(false);
        setError(errorMessage(e));
      });
  };

  const submit = async () => {
    if (!token) return;
    if (files.length < 1) {
      setError("Select at least one photo");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const result = await createPin({
        lat,
        lng,
        categoryId,
        caption: caption.trim() || null,
        photos: files,
      });
      onCreated(result.pin, result.photos);
      setFiles([]);
      setCaption("");
      setError(null);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      {posting && !open && (
        <button
          type="button"
          onClick={() => onOpenChange(true)}
          className="absolute bottom-6 right-6 z-10 flex h-14 w-14 items-center justify-center rounded-full bg-rose-600 text-3xl text-white shadow-lg transition-transform hover:scale-105"
          aria-label="Create a pin"
        >
          +
        </button>
      )}

      <div
        role="dialog"
        aria-label="Create a pin"
        className={`absolute z-40 flex flex-col bg-white shadow-2xl shadow-zinc-900/20 transition-transform duration-300 ease-out will-change-transform
          inset-x-0 bottom-0 max-h-[85dvh] rounded-t-2xl
          sm:left-0 sm:right-auto sm:top-16 sm:bottom-0 sm:max-h-none sm:w-full sm:max-w-md sm:rounded-none sm:rounded-r-2xl
          dark:bg-zinc-900 dark:shadow-zinc-950/60
          ${
            open
              ? "translate-y-0 sm:translate-y-0 sm:translate-x-0"
              : "translate-y-full sm:translate-y-0 sm:-translate-x-full"
          }`}
      >
        <header className="flex items-center justify-between border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
          <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            {isLoggedIn ? "Create a pin" : "Sign in required"}
          </h2>
          <button
            type="button"
            onClick={handleClose}
            className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
            aria-label="Close"
          >
            <CloseIcon />
          </button>
        </header>

        <div className="flex-1 overflow-y-auto p-5">
          {!isLoggedIn ? (
            <div className="space-y-3 text-sm text-zinc-600 dark:text-zinc-400">
              <p>
                You need to sign in before you can post a pin to the map.
              </p>
              <div className="flex gap-2">
                <Link
                  href="/login"
                  className="flex-1 rounded-lg bg-rose-600 px-4 py-2 text-center font-medium text-white hover:bg-rose-700"
                >
                  Log in
                </Link>
                <Link
                  href="/register"
                  className="flex-1 rounded-lg border border-zinc-300 px-4 py-2 text-center font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                >
                  Sign up
                </Link>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <button
                type="button"
                onClick={() => inputRef.current?.click()}
                className="flex w-full flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-zinc-300 py-8 text-zinc-500 hover:border-rose-400 hover:text-rose-600 dark:border-zinc-700 dark:text-zinc-400"
              >
                <span className="text-sm font-medium">
                  {files.length > 0
                    ? `${files.length} photo${files.length > 1 ? "s" : ""} selected`
                    : "Tap to add photos"}
                </span>
                <span className="text-xs">JPG/PNG, up to 5 photos, 10MB each</span>
              </button>
              <input
                ref={inputRef}
                type="file"
                accept="image/jpeg,image/png"
                multiple
                className="hidden"
                onChange={(e) => handleFiles(e.target.files)}
              />

              {files.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {files.map((f, i) => (
                    <div
                      key={`${f.name}-${i}`}
                      className="group relative h-20 w-20 overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700"
                    >
                      <img
                        src={previews[i]}
                        alt={f.name}
                        className="h-full w-full object-cover"
                      />
                      <button
                        type="button"
                        onClick={() => removeFile(i)}
                        className="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100"
                        aria-label={`Remove ${f.name}`}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>
              )}

              <div className="flex items-center justify-between gap-2 rounded-lg border border-zinc-200 px-3 py-2 text-sm dark:border-zinc-700">
                <span className="truncate text-zinc-500 dark:text-zinc-400">
                  Posting at {lat.toFixed(5)}, {lng.toFixed(5)}
                </span>
                <button
                  type="button"
                  onClick={useCurrentLocation}
                  disabled={locating}
                  className="shrink-0 font-medium text-rose-600 hover:underline disabled:opacity-60"
                >
                  {locating ? "Locating…" : "Use current location"}
                </button>
              </div>

              <select
                value={categoryId}
                onChange={(e) => setCategoryId(Number(e.target.value))}
                className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
              >
                {categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>

              <input
                type="text"
                value={caption}
                onChange={(e) => setCaption(e.target.value)}
                maxLength={200}
                placeholder="Add a caption (optional)"
                className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
              />

              {error && (
                <p className="text-sm text-rose-600 dark:text-rose-400" role="alert">
                  {error}
                </p>
              )}

              <button
                type="button"
                onClick={submit}
                disabled={submitting}
                className="w-full rounded-lg bg-rose-600 px-4 py-2.5 font-medium text-white hover:bg-rose-700 disabled:opacity-60"
              >
                {submitting ? "Posting…" : "Post to map"}
              </button>
            </div>
          )}
        </div>
      </div>
    </>
  );
}