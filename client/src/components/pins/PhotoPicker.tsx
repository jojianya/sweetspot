"use client";

import { useRef } from "react";
import { PhotoThumbGrid } from "@/components/pins/PhotoThumbGrid";
import { PHOTO_ACCEPT } from "@/lib/utils/photoValidation";

/** Dashed photo-picker button, hidden file input and thumbnail previews. */
export function PhotoPicker({
  files,
  previews,
  onFiles,
  onRemove,
  title,
  hint,
  inputAriaLabel,
}: {
  files: File[];
  previews: string[];
  onFiles: (list: FileList | null) => void;
  onRemove: (index: number) => void;
  title: string;
  hint: string;
  inputAriaLabel: string;
}) {
  const inputRef = useRef<HTMLInputElement>(null);

  return (
    <>
      <button
        type="button"
        onClick={() => inputRef.current?.click()}
        className="flex w-full flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed border-zinc-300 py-8 text-zinc-500 hover:border-rose-400 hover:text-rose-600 dark:border-zinc-700 dark:text-zinc-400"
      >
        <span className="text-sm font-medium">
          {files.length > 0
            ? `${files.length} photo${files.length > 1 ? "s" : ""} selected`
            : title}
        </span>
        <span className="text-xs">{hint}</span>
      </button>
      <input
        ref={inputRef}
        type="file"
        accept={PHOTO_ACCEPT}
        multiple
        className="hidden"
        aria-label={inputAriaLabel}
        onChange={(e) => onFiles(e.target.files)}
      />

      {files.length > 0 && (
        <PhotoThumbGrid files={files} previews={previews} onRemove={onRemove} />
      )}
    </>
  );
}
