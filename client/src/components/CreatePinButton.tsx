"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { createPin, type CreatedPin } from "@/lib/api";
import type { NewPinPhoto } from "@/lib/types";
import { useAuth } from "@/store/auth";

const MAX_PHOTOS = 5;
const MAX_PHOTO_SIZE = 10 * 1024 * 1024;

interface CreatePinButtonProps {
  lat: number;
  lng: number;
  categories: { id: number; name: string }[];
  onCreated: (
    pin: CreatedPin,
    photos: NewPinPhoto[]
  ) => void;
}

export default function CreatePinButton({
  lat,
  lng,
  categories,
  onCreated,
}: CreatePinButtonProps) {
  const { token, isLoggedIn } = useAuth();
  const [open, setOpen] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const [caption, setCaption] = useState("");
  const [categoryId, setCategoryId] = useState<number>(categories[0]?.id ?? 1);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

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

  const submit = async () => {
    if (!token) return;
    if (files.length < 1) {
      setError("Select at least one photo");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const result = await createPin(token, {
        lat,
        lng,
        categoryId,
        caption: caption.trim() || null,
        photos: files,
      });
      onCreated(result.pin, result.photos);
      setOpen(false);
      setFiles([]);
      setCaption("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="absolute bottom-6 right-6 z-10 flex h-14 w-14 items-center justify-center rounded-full bg-rose-600 text-3xl text-white shadow-lg transition-transform hover:scale-105"
        aria-label="Create a pin"
      >
        +
      </button>

      {open && (
        <div className="absolute inset-0 z-20 flex items-center justify-center bg-black/40 p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold text-zinc-900">
                {isLoggedIn ? "Create a pin" : "Sign in required"}
              </h2>
              <button
                type="button"
                onClick={() => setOpen(false)}
                className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100"
                aria-label="Close"
              >
                <svg
                  className="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M6 18 18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>

            {!isLoggedIn ? (
              <div className="space-y-3 text-sm text-zinc-600">
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
                    className="flex-1 rounded-lg border border-zinc-300 px-4 py-2 text-center font-medium text-zinc-700 hover:bg-zinc-50"
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
                  className="flex w-full flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-zinc-300 py-8 text-zinc-500 hover:border-rose-400 hover:text-rose-600"
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
                        className="flex items-center gap-2 rounded-full bg-zinc-100 px-3 py-1 text-xs text-zinc-600"
                      >
                        {f.name}
                        <span className="text-zinc-400">
                          {(f.size / 1024 / 1024).toFixed(1)}MB
                        </span>
                      </div>
                    ))}
                  </div>
                )}

                <select
                  value={categoryId}
                  onChange={(e) => setCategoryId(Number(e.target.value))}
                  className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900"
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
                  className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400"
                />

                {error && (
                  <p className="text-sm text-rose-600" role="alert">
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
      )}
    </>
  );
}