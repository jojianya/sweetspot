"use client";

import { useState } from "react";
import type { PinDetail } from "@/lib/types";
import { formatTime } from "@/lib/utils";

interface PinDetailPanelProps {
  pin: PinDetail;
  onClose: () => void;
}

export default function PinDetailPanel({ pin, onClose }: PinDetailPanelProps) {
  const [index, setIndex] = useState(0);
  const photo = pin.photos[index];
  const count = pin.photos.length;

  return (
    <div className="absolute inset-y-0 right-0 z-10 flex w-full max-w-sm flex-col bg-white shadow-xl">
      <header className="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
        <h2 className="text-lg font-semibold text-zinc-900">
          {pin.category ?? "Pin"}
        </h2>
        <button
          type="button"
          onClick={onClose}
          className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 hover:text-zinc-700"
          aria-label="Close"
        >
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
          </svg>
        </button>
      </header>

      <div className="flex-1 overflow-y-auto">
        {photo ? (
          <div className="relative">
            <img
              src={photo.photo_url}
              alt={pin.caption ?? "Pin photo"}
              className="aspect-square w-full object-cover"
            />
            {count > 1 && (
              <>
                <button
                  type="button"
                  onClick={() => setIndex((i) => (i - 1 + count) % count)}
                  className="absolute left-2 top-1/2 -translate-y-1/2 rounded-full bg-black/50 p-2 text-white hover:bg-black/70"
                  aria-label="Previous photo"
                >
                  <svg
                    className="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
                  </svg>
                </button>
                <button
                  type="button"
                  onClick={() => setIndex((i) => (i + 1) % count)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded-full bg-black/50 p-2 text-white hover:bg-black/70"
                  aria-label="Next photo"
                >
                  <svg
                    className="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                  </svg>
                </button>
                <span className="absolute bottom-2 right-2 rounded bg-black/60 px-2 py-0.5 text-xs text-white">
                  {index + 1}/{count}
                </span>
              </>
            )}
          </div>
        ) : (
          <div className="flex aspect-square items-center justify-center bg-zinc-100 text-zinc-400">
            No photo
          </div>
        )}

        <div className="space-y-3 p-4">
          <div className="flex flex-wrap gap-2">
            {pin.photos.map((p, i) => (
              <button
                key={p.id}
                type="button"
                onClick={() => setIndex(i)}
                className={`h-16 w-16 overflow-hidden rounded-lg border-2 ${
                  i === index ? "border-rose-600" : "border-transparent"
                }`}
              >
                <img
                  src={p.thumbnail_url}
                  alt={`Thumbnail ${i + 1}`}
                  className="h-full w-full object-cover"
                />
              </button>
            ))}
          </div>

          <p className="text-xl font-semibold leading-tight text-zinc-900">
            {pin.caption ?? "Untitled"}
          </p>

          <div className="flex items-center justify-between text-sm text-zinc-500">
            <span className="font-medium text-zinc-700">@{pin.username ?? "unknown"}</span>
            <time dateTime={pin.created_at}>{formatTime(pin.created_at)}</time>
          </div>
        </div>
      </div>
    </div>
  );
}