"use client";

import type { Category } from "@/lib/types";

/** Category select, caption field, error line and submit button. */
export function CreatePinForm({
  categories,
  selectedCategoryId,
  onCategoryChange,
  caption,
  onCaptionChange,
  error,
  submitting,
  onSubmit,
}: {
  categories: Category[];
  selectedCategoryId: number | null;
  onCategoryChange: (id: number) => void;
  caption: string;
  onCaptionChange: (value: string) => void;
  error: string | null;
  submitting: boolean;
  onSubmit: () => void;
}) {
  return (
    <div className="space-y-4">
      <select
        value={selectedCategoryId ?? ""}
        onChange={(e) => onCategoryChange(Number(e.target.value))}
        disabled={categories.length === 0}
        aria-label="Category"
        className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
      >
        {selectedCategoryId === null && <option value="">Choose a category</option>}
        {categories.map((c) => (
          <option key={c.id} value={c.id}>
            {c.name}
          </option>
        ))}
      </select>

      <input
        type="text"
        value={caption}
        onChange={(e) => onCaptionChange(e.target.value)}
        maxLength={200}
        placeholder="Add a caption (optional)"
        aria-label="Add a caption"
        className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
      />

      {error && (
        <p className="text-sm text-rose-600 dark:text-rose-400" role="alert">
          {error}
        </p>
      )}

      <button
        type="button"
        onClick={onSubmit}
        disabled={submitting || selectedCategoryId === null}
        className="w-full rounded-lg bg-rose-600 px-4 py-2.5 font-medium text-white hover:bg-rose-700 disabled:opacity-60"
      >
        {submitting ? "Posting…" : "Post to map"}
      </button>
    </div>
  );
}
