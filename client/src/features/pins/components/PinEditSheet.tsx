"use client";

import { useEffect, useRef, useState } from "react";
import PanelSheet from "@/shared/ui/PanelSheet";
import { SheetHeader } from "@/features/pins/components/SheetChrome";
import { deletePin, updatePin } from "@/features/pins/api/pins";
import { errorMessage } from "@/shared/utils";
import type { Category } from "@/shared/types/category"
import type { PinDetail } from "@/shared/types/pin";
import { resolveMediaUrl } from "@/shared/lib/media";
import { usePhotoPreviews } from "@/features/photos/hooks/usePhotoPreviews";
import { PhotoThumbGrid } from "@/features/photos/components/PhotoThumbGrid";
import { MAX_PIN_PHOTOS, PHOTO_ACCEPT, validatePhotoFiles } from "@/features/photos/lib/photoValidation";

interface PinEditSheetProps {
  pin: PinDetail;
  categories: Category[];
  onClose: () => void;
  onUpdated: (pin: PinDetail) => void;
  onDeleted: (id: string) => void;
}

/** Edit sheet for a pin the viewer owns: caption, category, photo swap. */
export default function PinEditSheet({
  pin,
  categories,
  onClose,
  onUpdated,
  onDeleted,
}: PinEditSheetProps) {
  const [caption, setCaption] = useState(pin.caption ?? "");
  const [categoryId, setCategoryId] = useState(pin.category_id);
  const [files, setFiles] = useState<File[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const confirmDeleteRef = useRef<HTMLButtonElement>(null);

  const previews = usePhotoPreviews(files);

  // Move focus to the destructive confirm action when it appears, so
  // keyboard and screen-reader users land on the decision point.
  useEffect(() => {
    if (confirmingDelete) confirmDeleteRef.current?.focus();
  }, [confirmingDelete]);

  const handleFiles = (list: FileList | null) => {
    if (!list) return;
    const picked = Array.from(list);
    const validationError = validatePhotoFiles(picked);
    if (validationError) {
      setError(validationError);
      return;
    }
    setError(null);
    setFiles(picked);
  };

  const submit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const result = await updatePin(pin.id, {
        caption: caption.trim(),
        categoryId,
        photos: files.length > 0 ? files : null,
      });
      // The update response carries the pin's photo set as it stands after the
      // edit, so adopt it instead of refetching the pin. category, username and
      // avatar_url are not editable here, so they carry over unchanged.
      onUpdated({ ...pin, ...result.pin, photos: result.photos });
      onClose();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSubmitting(false);
    }
  };

  const destroy = async () => {
    setDeleting(true);
    setError(null);
    try {
      await deletePin(pin.id);
      onDeleted(pin.id);
      onClose();
    } catch (e) {
      setError(errorMessage(e));
      setConfirmingDelete(false);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <PanelSheet role="dialog" aria-modal="true" aria-label="Edit pin" onClose={onClose}>
      <SheetHeader title="Edit pin" onClose={onClose} />

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
                {pin.photos.map((p) => {
                  const src =
                    resolveMediaUrl(p.thumbnail_url) || resolveMediaUrl(p.photo_url);
                  if (!src) return null;
                  return (
                    <img
                      key={p.id}
                      src={src}
                      alt=""
                      className="h-16 w-16 shrink-0 rounded-lg border border-zinc-200 object-cover dark:border-zinc-700"
                    />
                  );
                })}
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
                picking photos replaces the current set (JPG/PNG, {MAX_PIN_PHOTOS} max)
              </span>
            </button>
            <input
              ref={inputRef}
              type="file"
              accept={PHOTO_ACCEPT}
              multiple
              className="hidden"
              aria-label="Replace photos"
              onChange={(e) => handleFiles(e.target.files)}
            />
          </div>

          {files.length > 0 && (
            <PhotoThumbGrid
              files={files}
              previews={previews}
              onRemove={(i) => setFiles((prev) => prev.filter((_, j) => j !== i))}
            />
          )}

          {error && (
            <p className="text-sm text-rose-600 dark:text-rose-400" role="alert">
              {error}
            </p>
          )}

          <button
            type="button"
            onClick={submit}
            disabled={submitting || deleting}
            className="w-full rounded-lg bg-rose-600 px-4 py-2.5 font-medium text-white hover:bg-rose-700 disabled:opacity-60"
          >
            {submitting ? "Saving…" : "Save changes"}
          </button>

          {confirmingDelete ? (
            <div
              className="rounded-lg border border-rose-200 bg-rose-50 p-3 dark:border-rose-900 dark:bg-rose-950/40"
              aria-label="Confirm pin deletion"
            >
              <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                Delete this pin? This cannot be undone.
              </p>
              <div className="mt-3 flex gap-2">
                <button
                  ref={confirmDeleteRef}
                  type="button"
                  onClick={destroy}
                  disabled={deleting}
                  className="flex-1 rounded-lg bg-rose-600 px-4 py-2 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-60"
                >
                  {deleting ? "Deleting…" : "Yes, delete pin"}
                </button>
                <button
                  type="button"
                  onClick={() => setConfirmingDelete(false)}
                  disabled={deleting}
                  className="flex-1 rounded-lg border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                >
                  Keep pin
                </button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setConfirmingDelete(true)}
              disabled={submitting || deleting}
              className="w-full rounded-lg px-4 py-2 text-sm font-medium text-rose-600 hover:bg-rose-50 disabled:opacity-60 dark:text-rose-400 dark:hover:bg-rose-950/40"
            >
              Delete pin
            </button>
          )}
        </div>
      </div>
    </PanelSheet>
  );
}