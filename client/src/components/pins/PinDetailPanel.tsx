"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import PanelSheet from "@/components/PanelSheet";
import { BookmarkIcon, CloseIcon } from "@/components/icons";
import PhotoLightbox from "./PhotoLightbox";
import CommentsSection from "./CommentsSection";
import PinEditSheet from "./PinEditSheet";
import AddToCollectionSheet from "./AddToCollectionSheet";
import type { PinDetail } from "@/lib/types";
import { parsePoint } from "@/lib/utils";
import { reverseGeocode } from "@/lib/api/geocoding";
import { fetchPin } from "@/lib/api/pins";
import { removeFavorite, saveFavorite } from "@/lib/api/favorites";
import { useCategories } from "@/hooks/useCategories";
import { useAuth } from "@/store/auth";
import { useSavedStatus } from "@/hooks/useFavorites";

interface PinDetailPanelProps {
  pin: PinDetail;
  onClose: () => void;
}

const stroke = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  viewBox: "0 0 24 24",
};

function DirectionsIcon() {
  return (
    <svg className="h-5 w-5" {...stroke} aria-hidden>
      <path d="M7 7h10v10" />
      <path d="M7 17 17 7" />
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

function ListIcon() {
  return (
    <svg className="h-4 w-4" {...stroke} aria-hidden>
      <path d="M8 6h12M8 12h12M8 18h12" />
      <path d="M3 6h.01M3 12h.01M3 18h.01" />
    </svg>
  );
}

function ChatIcon() {
  return (
    <svg className="h-4 w-4" {...stroke} aria-hidden>
      <path d="M21 12a8 8 0 0 1-8 8H4l2.5-2.5A8 8 0 1 1 21 12Z" />
    </svg>
  );
}

export default function PinDetailPanel({ pin: initialPin, onClose }: PinDetailPanelProps) {
  const router = useRouter();
  const { token, user } = useAuth();
  const { categories } = useCategories();
  const [pin, setPin] = useState<PinDetail>(initialPin);
  const [index, setIndex] = useState(0);
  const [lightbox, setLightbox] = useState(false);
  const [copied, setCopied] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [collectionOpen, setCollectionOpen] = useState(false);
  const [commentsOpen, setCommentsOpen] = useState(false);
  const { saved, setSaved } = useSavedStatus(pin.id, token !== null);

  const photo = pin.photos[index];
  const count = pin.photos.length;
  const point = useMemo(() => parsePoint(pin.location), [pin.location]);

  const isOwner =
    user !== null && (user.id === pin.user_id || user.role === "admin" || user.role === "owner");

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
    const url = `${window.location.origin}/pin/${pin.id}`;
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

  const handleUpdated = async () => {
    try {
      const refreshed = await fetchPin(pin.id);
      setPin(refreshed);
    } catch {
      // ignored: the sheet already confirmed the update server-side
    }
  };

  const name = pin.caption?.trim();

  return (
    <>
      <PanelSheet role="dialog" aria-label="Pin details">
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
          <div className="flex items-start justify-between gap-2">
            <h2 className="text-xl font-bold leading-tight text-zinc-900 dark:text-zinc-100">
              {name ?? "Untitled"}
            </h2>
            <button
              type="button"
              onClick={onClose}
              className="shrink-0 rounded-full p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-600 dark:hover:bg-zinc-800 dark:hover:text-zinc-300"
              aria-label="Close"
            >
              <CloseIcon />
            </button>
          </div>

          <div className="mt-0.5 flex items-center gap-1.5 text-sm text-zinc-500 dark:text-zinc-400">
            {pin.category && (
              <>
                <span className="text-zinc-600 dark:text-zinc-300">{pin.category}</span>
                <span className="text-zinc-300 dark:text-zinc-600">·</span>
              </>
            )}
            {pin.username && (
              <Link
                href={pin.user_id ? `/users/${pin.user_id}` : "#"}
                className="flex min-w-0 items-center gap-1.5 hover:underline"
              >
                <PersonIcon />
                <span className="truncate">{pin.username}</span>
              </Link>
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

          <div className="mt-3 flex items-start gap-2 text-sm text-zinc-700 dark:text-zinc-300">
            <span className="mt-0.5 shrink-0 text-zinc-400 dark:text-zinc-500">
              <PinIcon />
            </span>
            <span>
              {address ?? (point ? "Finding address…" : "On the GoodSpot map")}
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

          {/* Secondary actions */}
          <div className="mt-5 flex flex-wrap items-center gap-2">
            {isOwner && (
              <button
                type="button"
                onClick={() => setEditOpen(true)}
                className="rounded-full border border-zinc-300 px-3 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Edit
              </button>
            )}
            <button
              type="button"
              onClick={() => setCollectionOpen(true)}
              className="flex items-center gap-1.5 rounded-full border border-zinc-300 px-3 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              <ListIcon />
              Add to collection
            </button>
            <Link
              href={`/pin/${pin.id}`}
              className="rounded-full border border-zinc-300 px-3 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Open page
            </Link>
          </div>

          {/* Comments */}
          <div className="mt-5 border-t border-zinc-100 pt-4 dark:border-zinc-800">
            <button
              type="button"
              onClick={() => setCommentsOpen((v) => !v)}
              className="flex w-full items-center justify-between text-sm font-semibold text-zinc-900 dark:text-zinc-100"
              aria-expanded={commentsOpen}
            >
              <span className="flex items-center gap-2">
                <span className="text-zinc-400">
                  <ChatIcon />
                </span>
                Comments
              </span>
              <span className="text-xs font-medium text-zinc-400">{commentsOpen ? "Hide" : "Show"}</span>
            </button>
            {commentsOpen && (
              <div className="mt-3">
                <CommentsSection pinId={pin.id} compact />
              </div>
            )}
          </div>
        </div>
      </PanelSheet>

      {lightbox && photo && (
        <PhotoLightbox
          src={photo.photo_url}
          alt={name ?? "Pin photo"}
          count={count}
          index={index}
          onNavigate={setIndex}
          onClose={() => setLightbox(false)}
        />
      )}

      {editOpen && (
        <PinEditSheet
          pin={pin}
          categories={categories.length > 0 ? categories : [{ id: pin.category_id, name: pin.category ?? "Other" }]}
          onClose={() => setEditOpen(false)}
          onUpdated={handleUpdated}
        />
      )}

      {collectionOpen && (
        <AddToCollectionSheet pinId={pin.id} onClose={() => setCollectionOpen(false)} />
      )}
    </>
  );
}