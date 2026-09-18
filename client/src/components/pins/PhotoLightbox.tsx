"use client";

import { useEffect } from "react";
import { CloseIcon } from "@/components/icons";

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  viewBox: "0 0 24 24",
};

type PhotoLightboxProps = {
  src: string;
  alt: string;
  /** Total number of photos; arrow navigation renders only when > 1. */
  count: number;
  index: number;
  onNavigate: (nextIndex: number) => void;
  onClose: () => void;
};

export default function PhotoLightbox({
  src,
  alt,
  count,
  index,
  onNavigate,
  onClose,
}: PhotoLightboxProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowRight" && count > 1) {
        onNavigate((index + 1) % count);
      } else if (e.key === "ArrowLeft" && count > 1) {
        onNavigate((index - 1 + count) % count);
      } else if (e.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [count, index, onNavigate, onClose]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={alt}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/95 p-4"
      onClick={onClose}
    >
      <img
        src={src}
        alt={alt}
        className="max-h-[85vh] max-w-full rounded-lg object-contain shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      />
      <button
        type="button"
        onClick={onClose}
        className="absolute right-4 top-4 rounded-full bg-white/10 p-2 text-white backdrop-blur hover:bg-white/20"
        aria-label="Close viewer"
      >
        <CloseIcon />
      </button>
      {count > 1 && (
        <>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onNavigate((index - 1 + count) % count);
            }}
            className="absolute left-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-3 text-white backdrop-blur hover:bg-white/20"
            aria-label="Previous photo"
          >
            <svg className="h-6 w-6" {...stroke} aria-hidden>
              <path d="M15 19l-7-7 7-7" />
            </svg>
          </button>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onNavigate((index + 1) % count);
            }}
            className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-white/10 p-3 text-white backdrop-blur hover:bg-white/20"
            aria-label="Next photo"
          >
            <svg className="h-6 w-6" {...stroke} aria-hidden>
              <path d="M9 5l7 7-7 7" />
            </svg>
          </button>
          <span className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full bg-white/10 px-3 py-1 text-sm text-white backdrop-blur">
            {index + 1} / {count}
          </span>
        </>
      )}
    </div>
  );
}
