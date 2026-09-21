"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import PanelSheet from "@/components/PanelSheet";
import { CloseIcon } from "@/components/icons";
import { fetchPin, updatePin } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { Category, PinDetail } from "@/lib/types";

const MAX_PHOTOS = 5;
const MAX_PHOTO_SIZE = 10 * 1024 * 1024;

interface PinEditSheetProps {
  pin: PinDetail;
  categories: Category[];
  onClose: () => void;
  onUpdated: (pin: PinDetail) => void;
}

/** Edit sheet for a pin the viewer owns: caption, category, photo swap. */
export default function PinEditSheet({
  pin,
  categories,
  onClose,
  onUpdated,
}: PinEditSheetProps) {
  const [caption, setCaption] = useState(pin.caption ?? "");
  const [categoryId, setCategoryId] = useState(pin.category_id);
  const [files, setFiles] = useState<File[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const previews = useMemo(() => files.map((f) => URL.createObjectURL(f)), [files]);

  useEffect(() => {
    return () => previews.forEach((u) => URL.revokeObjectURL(u));
  }, [previews]);

  useEffect(() => {
    if (files.length > 0) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [files.length, onClose]);

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
    setSubmitting(true);
    setError(null);
    try {
      await updatePin(pin.id, {
        caption: caption.trim(),
        categoryId,
        photos: files.length > 0 ? files : null,
      });
      const refreshed = await fetchPin(pin.id);
      onUpdated(refreshed);
      onClose();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PanelSheet role="dialog" aria-modal="true" aria-label="Edit pin">
      <header className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
        <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
          Edit pin
        </h2>
        <button
          type="button"
          onClick={onClose}
          className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
          aria-label="Close"
        >
          <CloseIcon />
        </button>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        <div className="space-y-4">
          <select
            value={categoryId}
            onChange={(e) => setCategoryId(Number(e.target.value))}
            className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
            aria-label="Category"
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
            aria-label="Caption"
          />

          {pin.photos.length > 0 && (
            <div>
              <p className="mb-2 text-xs font-medium text-zinc-500 dark:text-zinc-400">
                Current photos
              </p>
              <div className="flex gap-2 overflow-x-auto pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                {pin.photos.map((p) => (
                  <img
                    key={p.id}
                    src={p.thumbnail_url}
                    alt=""
                    className="h-16 w-16 shrink-0 rounded-lg border border-zinc-200 object-cover dark:border-zinc-700"
                  />
                ))}
              </div>
            </div>
          )}

          <div>
            <button
              type="button"
              onClick={() => inputRef.current?.click()}
              className="flex w-full flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-zinc-300 py-6 text-zinc-500 hover:border-rose-400 hover:text-rose-600 dark:border-zinc-700 dark:text-zinc-400"
            >
              <span className="text-sm font-medium">
                {files.length > 0
                  ? `${files.length} new photo${files.length > 1 ? "s" : ""} selected`
                  : "Replace photos"}
              </span>
              <span className="text-xs">
                picking photos replaces the current set (JPG/PNG, {MAX_PHOTOS} max)
              </span>
            </button>
            <input
              ref={inputRef}
              type="file"
              accept="image/jpeg,image/png"
              multiple
              className="hidden"
              onChange={(e) => handleFiles(e.target.files)}
            />
          </div>

          {files.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {files.map((f, i) => (
                <div
                  key={`${f.name}-${i}`}
                  className="group relative h-20 w-20 overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700"
                >
                  <img src={previews[i]} alt={f.name} className="h-full w-full object-cover" />
                  <button
                    type="button"
                    onClick={() => setFiles((prev) => prev.filter((_, j) => j !== i))}
                    className="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100"
                    aria-label={`Remove ${f.name}`}
                  >
                    ×
                  </button>
                </div>
              ))}
            </div>
          )}

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
            {submitting ? "Saving…" : "Save changes"}
          </button>
        </div>
      </div>
    </PanelSheet>
  );
}