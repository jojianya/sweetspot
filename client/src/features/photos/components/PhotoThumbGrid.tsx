"use client";

/**
 * Thumbnail grid for picked-but-unsubmitted photos, with a per-photo remove
 * button. Shared by the create and edit sheets.
 */
export function PhotoThumbGrid({
  files,
  previews,
  onRemove,
}: {
  files: File[];
  previews: string[];
  onRemove: (index: number) => void;
}) {
  if (files.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-2">
      {files.map((f, i) => (
        <div
          key={`${f.name}-${i}`}
          className="group relative h-20 w-20 overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700"
        >
          <img src={previews[i]} alt={f.name} className="h-full w-full object-cover" />
          <button
            type="button"
            onClick={() => onRemove(i)}
            className="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100"
            aria-label={`Remove ${f.name}`}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
