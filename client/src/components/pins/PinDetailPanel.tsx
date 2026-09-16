"use client";

import { useEffect, useState } from "react";
import type { PinDetail } from "@/lib/types";
import { formatTime } from "@/lib/utils";

interface PinDetailPanelProps {
  pin: PinDetail;
  currentUserId: string | null;
  onDelete: (id: string) => Promise<void>;
  onClose: () => void;
}

export default function PinDetailPanel({
  pin,
  currentUserId,
  onDelete,
  onClose,
}: PinDetailPanelProps) {
  const [index, setIndex] = useState(0);
  const [lightbox, setLightbox] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const photo = pin.photos[index];
  const count = pin.photos.length;
  const isOwner = pin.user_id !== "" && pin.user_id === currentUserId;

  useEffect(() => {
    if (!lightbox) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowRight" && count > 1) {
        setIndex((i) => (i + 1) % count);
      } else if (e.key === "ArrowLeft" && count > 1) {
        setIndex((i) => (i - 1 + count) % count);
      } else if (e.key === "Escape") {
        setLightbox(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [lightbox, count]);

  const handleDelete = async () => {
    if (deleting) return;
    if (!window.confirm("Delete this pin? This can't be undone.")) return;
    setDeleting(true);
    try {
      await onDelete(pin.id);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <>
      <div className="absolute inset-y-0 right-0 z-10 flex w-full max-w-sm flex-col bg-white shadow-2xl shadow-zinc-900/20">
        <header className="flex items-center justify-between px-4 py-3">
          <h2 className="text-lg font-semibold text-zinc-900">Pin</h2>
          <div className="flex items-center gap-1">
            {isOwner && (
              <button
                type="button"
                onClick={handleDelete}
                disabled={deleting}
                className="rounded-full p-1.5 text-zinc-400 transition-colors hover:bg-rose-50 hover:text-rose-600"
                aria-label="Delete pin"
                title="Delete pin"
              >
                <svg
                  className={"h-5 w-5" + (deleting ? " animate-pulse" : "")}
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6h14Z"
                  />
                </svg>
              </button>
            )}
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
          </div>
        </header>

        <div className="flex-1 overflow-y-auto pb-4">
          {photo ? (
            <button
              type="button"
              onClick={() => setLightbox(true)}
              className="relative block w-full cursor-zoom-in"
              aria-label="View photo full screen"
            >
              <img
                src={photo.photo_url}
                alt={pin.caption ?? "Pin photo"}
                loading="lazy"
                decoding="async"
                className="aspect-square w-full bg-zinc-100 object-cover"
              />
              {pin.category && (
                <span className="absolute left-3 top-3 rounded-full bg-black/60 px-2.5 py-1 text-xs font-medium text-white backdrop-blur">
                  {pin.category}
                </span>
              )}
            </button>
          ) : (
            <div className="flex aspect-square items-center justify-center bg-zinc-100 text-zinc-400">
              No photo
            </div>
          )}

          {count > 1 && (
            <div className="flex flex-wrap gap-2 px-4 pt-3">
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
                    loading="lazy"
                    decoding="async"
                    className="h-full w-full object-cover"
                  />
                </button>
              ))}
            </div>
          )}

          <div className="space-y-3 px-4 pt-4">
            <p className="text-xl font-semibold leading-tight text-zinc-900">
              {pin.caption ?? "Untitled"}
            </p>

            <div className="flex items-center gap-2.5">
              {pin.avatar_url ? (
                <img
                  src={pin.avatar_url}
                  alt=""
                  className="h-8 w-8 rounded-full object-cover"
                />
              ) : (
                <span className="flex h-8 w-8 items-center justify-center rounded-full bg-rose-100 text-sm font-semibold text-rose-700">
                  {pin.username?.charAt(0).toUpperCase() ?? "?"}
                </span>
              )}
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-zinc-700">
                  @{pin.username ?? "unknown"}
                </p>
                <time className="text-xs text-zinc-400" dateTime={pin.created_at}>
                  {formatTime(pin.created_at)}
                </time>
              </div>
            </div>
          </div>
        </div>
      </div>

      {lightbox && photo && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={pin.caption ?? "Pin photo"}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/95 p-4"
          onClick={() => setLightbox(false)}
        >
          <img
            src={photo.photo_url}
            alt={pin.caption ?? "Pin photo"}
            className="max-h-[85vh] max-w-full rounded-lg object-contain shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          />
          <button
            type="button"
            onClick={() => setLightbox(false)}
            className="absolute right-4 top-4 rounded-full bg-white/10 p-2 text-white backdrop-blur hover:bg-white/20"
            aria-label="Close viewer"
          >
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" d="M6 18 18 6M6 6l12 12" />
            </svg>
          </button>
          {count > 1 && (
            <>
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setIndex((i) => (i - 1 + count) % count);
                }}
                className="absolute left-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-3 text-white backdrop-blur hover:bg-white/20"
                aria-label="Previous photo"
              >
                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
                </svg>
              </button>
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setIndex((i) => (i + 1) % count);
                }}
                className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-3 text-white backdrop-blur hover:bg-white/20"
                aria-label="Next photo"
              >
                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                </svg>
              </button>
              <span className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full bg-white/10 px-3 py-1 text-sm text-white backdrop-blur">
                {index + 1} / {count}
              </span>
            </>
          )}
        </div>
      )}
    </>
  );
}