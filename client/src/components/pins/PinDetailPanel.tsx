"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { PinDetail } from "@/lib/types";
import { parsePoint } from "@/lib/utils";
import { reverseGeocode } from "@/lib/api/geocoding";
import { fetchFavoriteIDs, removeFavorite, saveFavorite } from "@/lib/api/favorites";
import { useAuth } from "@/store/auth";

interface PinDetailPanelProps {
  pin: PinDetail;
  onClose: () => void;
}

function hashId(id: string): number {
  let h = 0;
  for (let i = 0; i < id.length; i += 1) {
    h = (h * 31 + id.charCodeAt(i)) >>> 0;
  }
  return h;
}

function StarIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d="M12 2.5 15 9l7 .6-5.3 4.7 1.6 6.9L12 17.6l-6.3 3.6 1.6-6.9L2 9.6 9 9Z" />
    </svg>
  );
}

function FillStar({ className, fill }: { className?: string; fill: number }) {
  const pct = Math.max(0, Math.min(1, fill)) * 100;
  return (
    <div className={"relative inline-block h-[18px] w-[18px] " + (className ?? "")}>
      <StarIcon className="absolute inset-0 h-full w-full text-zinc-200 dark:text-zinc-700" />
      <div className="absolute inset-0 overflow-hidden" style={{ width: `${pct}%` }}>
        <StarIcon className="h-[18px] w-[18px] text-amber-400" />
      </div>
    </div>
  );
}

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  viewBox: "0 0 24 24",
};

function CloseIcon() {
  return (
    <svg className="h-5 w-5" {...stroke} aria-hidden>
      <path d="M6 18 18 6M6 6l12 12" />
    </svg>
  );
}

function DirectionsIcon() {
  return (
    <svg className="h-5 w-5" {...stroke} aria-hidden>
      <path d="M7 7h10v10" />
      <path d="M7 17 17 7" />
    </svg>
  );
}

function BookmarkIcon({ filled }: { filled: boolean }) {
  return (
    <svg className="h-5 w-5" viewBox="0 0 24 24" fill={filled ? "currentColor" : "none"} stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M19 21 12 16 5 21V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2Z" />
    </svg>
  );
}

function ShareIcon() {
  return (
    <svg className="h-5 w-5" {...stroke} aria-hidden>
      <circle cx="6" cy="12" r="2.6" />
      <circle cx="18" cy="6" r="2.6" />
      <circle cx="18" cy="18" r="2.6" />
      <path d="m8.6 10.9 6.8-3.8M8.6 13.1l6.8 3.8" />
    </svg>
  );
}

function PinIcon() {
  return (
    <svg className="h-4 w-4" {...stroke} aria-hidden>
      <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z" />
      <circle cx="12" cy="10" r="3" />
    </svg>
  );
}

function PersonIcon() {
  return (
    <svg className="h-4 w-4" {...stroke} aria-hidden>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21c0-4 3.6-6.5 8-6.5s8 2.5 8 6.5" />
    </svg>
  );
}

export default function PinDetailPanel({ pin, onClose }: PinDetailPanelProps) {
  const router = useRouter();
  const { token } = useAuth();
  const [index, setIndex] = useState(0);
  const [lightbox, setLightbox] = useState(false);
  const [copied, setCopied] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const photo = pin.photos[index];
  const count = pin.photos.length;
  const point = useMemo(() => parsePoint(pin.location), [pin.location]);

  const rating = useMemo(() => {
    const value = 3.7 + (hashId(pin.id) % 13) / 10;
    return { value, reviews: 18 + (hashId(pin.id + ":r") % 183) };
  }, [pin.id]);

  const description = useMemo(() => {
    const base = `A ${pin.category ? pin.category.toLowerCase() : "neighbourhood"} favourite shared on GoodSpot.`;
    return count > 0
      ? `${base} Browse ${count} photo${count === 1 ? "" : "s"} in the gallery.`
      : base;
  }, [pin.category, count]);

  const [address, setAddress] = useState<string | null>(null);

  useEffect(() => {
    if (!point) return;
    const controller = new AbortController();
    reverseGeocode(point, controller.signal)
      .then((text) => {
        if (!controller.signal.aborted) setAddress(text);
      })
      .catch(() => {
        // lookup failed; the address row falls back to a neutral label
      });
    return () => controller.abort();
  }, [point]);

  useEffect(() => {
    let cancelled = false;
    if (!token) return;
    fetchFavoriteIDs()
      .then((ids) => {
        if (!cancelled) setSaved(ids.includes(pin.id));
      })
      .catch(() => {
        // saved state stays false if the lookup fails
      });
    return () => {
      cancelled = true;
    };
  }, [token, pin.id]);

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

  const handleSave = () => {
    if (!token) {
      router.push("/login");
      return;
    }
    if (saving) return;
    setSaving(true);
    const next = !saved;
    setSaved(next);
    const op = next ? saveFavorite(pin.id) : removeFavorite(pin.id);
    op.then(() => setSaving(false)).catch(() => {
      setSaved(!next);
      setSaving(false);
    });
  };

  const handleShare = async () => {
    const text = `Check out ${pin.caption ?? "this place"} on GoodSpot`;
    const url = window.location.href;
    if (navigator.share) {
      try {
        await navigator.share({ title: text, text, url });
        return;
      } catch {
        // user dismissed the share sheet; fall through to clipboard
      }
    }
    try {
      await navigator.clipboard.writeText(`${text} ${url}`);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // ignore clipboard failures
    }
  };

  const goDirections = () => {
    if (!point) return;
    window.open(
      `https://www.google.com/maps/dir/?api=1&destination=${point.lat},${point.lng}`,
      "_blank",
      "noopener,noreferrer"
    );
  };

  const name = pin.caption?.trim();

  return (
    <>
      <div className="absolute z-40 flex flex-col bg-white shadow-2xl shadow-zinc-900/20
          inset-x-0 bottom-0 max-h-[85dvh] rounded-t-2xl
          sm:left-0 sm:right-auto sm:top-16 sm:bottom-0 sm:max-h-none sm:w-full sm:max-w-md sm:rounded-none sm:rounded-r-2xl
          dark:bg-zinc-900 dark:shadow-zinc-950/60">
        {/* Hero photo — plain image, no overlays */}
        <div className="relative h-[210px] shrink-0 overflow-hidden rounded-t-2xl sm:rounded-t-none sm:rounded-tr-2xl">
          {photo ? (
            <button
              type="button"
              onClick={() => setLightbox(true)}
              className="block h-full w-full cursor-zoom-in"
              aria-label="View photo full screen"
            >
              <img
                src={photo.photo_url}
                alt={name ?? "Pin photo"}
                loading="lazy"
                decoding="async"
                className="h-full w-full object-cover"
              />
            </button>
          ) : (
            <div className="flex h-full w-full items-center justify-center bg-zinc-100 text-zinc-300 dark:bg-zinc-800 dark:text-zinc-600">
              <PinIcon />
            </div>
          )}
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-4 py-4">
          <h2 className="text-xl font-bold leading-tight text-zinc-900 dark:text-zinc-100">
            {name ?? "Untitled"}
          </h2>

          <div className="mt-1.5 flex items-center gap-2">
            <span className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">{rating.value.toFixed(1)}</span>
            <span className="flex items-center gap-0.5" aria-label={`${rating.value.toFixed(1)} out of 5`}>
              {[0, 1, 2, 3, 4].map((i) => (
                <FillStar key={i} fill={rating.value - i} />
              ))}
            </span>
            <span className="text-sm text-zinc-500 dark:text-zinc-400">({rating.reviews} reviews)</span>
          </div>

          <div className="mt-0.5 flex items-center gap-1.5 text-sm text-zinc-500 dark:text-zinc-400">
            {pin.category && (
              <>
                <span className="text-zinc-600 dark:text-zinc-300">{pin.category}</span>
                <span className="text-zinc-300 dark:text-zinc-600">·</span>
              </>
            )}
            {pin.username && (
              <>
                <PersonIcon />
                <span className="truncate">{pin.username}</span>
              </>
            )}
          </div>

          {/* Actions */}
          <div className="mt-5 flex items-start justify-around">
            <div className="flex flex-col items-center gap-1.5">
              <button
                type="button"
                onClick={goDirections}
                disabled={!point}
                className="flex h-11 w-11 items-center justify-center rounded-full bg-sky-500/10 text-sky-600 transition-colors hover:bg-sky-500/20 disabled:opacity-40 dark:bg-sky-500/20 dark:text-sky-400"
                aria-label="Directions"
              >
                <DirectionsIcon />
              </button>
              <span className="text-xs font-medium text-zinc-700 dark:text-zinc-300">Directions</span>
            </div>
            <div className="flex flex-col items-center gap-1.5">
              <button
                type="button"
                onClick={handleSave}
                disabled={saving}
                className="flex h-11 w-11 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-600 transition-colors hover:bg-emerald-500/20 disabled:opacity-40 dark:bg-emerald-500/20 dark:text-emerald-400"
                aria-label="Save"
                aria-pressed={saved}
              >
                <BookmarkIcon filled={saved} />
              </button>
              <span className="text-xs font-medium text-zinc-700 dark:text-zinc-300">{saved ? "Saved" : "Save"}</span>
            </div>
            <div className="flex flex-col items-center gap-1.5">
              <button
                type="button"
                onClick={handleShare}
                className="flex h-11 w-11 items-center justify-center rounded-full bg-amber-500/10 text-amber-600 transition-colors hover:bg-amber-500/20 dark:bg-amber-500/20 dark:text-amber-400"
                aria-label="Share"
              >
                <ShareIcon />
              </button>
              <span className="text-xs font-medium text-zinc-700 dark:text-zinc-300">{copied ? "Copied" : "Share"}</span>
            </div>
          </div>

          <div className="my-5 h-px bg-zinc-100 dark:bg-zinc-800" />

          <p className="text-sm leading-relaxed text-zinc-600 dark:text-zinc-400">{description}</p>

          <div className="mt-3 flex items-start gap-2 text-sm text-zinc-700 dark:text-zinc-300">
            <span className="mt-0.5 shrink-0 text-zinc-400 dark:text-zinc-500">
              <PinIcon />
            </span>
            <span>
              {address ??
                (point ? "Finding address…" : "On the GoodSpot map")}
            </span>
          </div>

          {count > 1 && (
            <div className="mt-5 border-t border-zinc-100 pt-4 dark:border-zinc-800">
              <p className="mb-2 text-sm font-semibold text-zinc-900 dark:text-zinc-100">Photos</p>
              <div className="flex gap-2 overflow-x-auto pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                {pin.photos.map((p, i) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => setIndex(i)}
                    aria-label={`Photo ${i + 1} of ${count}`}
                    aria-pressed={i === index}
                    className={`h-16 w-16 shrink-0 overflow-hidden rounded-xl border-2 transition-colors ${
                      i === index ? "border-sky-500" : "border-transparent hover:border-zinc-300 dark:hover:border-zinc-600"
                    }`}
                  >
                    <img
                      src={p.thumbnail_url}
                      alt=""
                      loading="lazy"
                      decoding="async"
                      className="h-full w-full object-cover"
                    />
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {lightbox && photo && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={name ?? "Pin photo"}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/95 p-4"
          onClick={() => setLightbox(false)}
        >
          <img
            src={photo.photo_url}
            alt={name ?? "Pin photo"}
            className="max-h-[85vh] max-w-full rounded-lg object-contain shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          />
          <button
            type="button"
            onClick={() => setLightbox(false)}
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
                  setIndex((i) => (i - 1 + count) % count);
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
                  setIndex((i) => (i + 1) % count);
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
      )}
    </>
  );
}