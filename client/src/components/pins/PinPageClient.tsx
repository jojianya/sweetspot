"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Navbar from "@/components/layout/Navbar";
import PhotoLightbox from "@/components/pins/PhotoLightbox";
import CommentsSection from "@/components/pins/CommentsSection";
import PinEditSheet from "@/components/pins/PinEditSheet";
import AddToCollectionSheet from "@/components/pins/AddToCollectionSheet";
import ReportSheet from "@/components/pins/ReportSheet";
import PinViews from "./PinViews";
import { BookmarkIcon, PersonIcon, PinIcon } from "@/components/icons";
import { fetchPin } from "@/lib/api";
import { useCategories } from "@/hooks/useCategories";
import { usePinAddress } from "@/hooks/usePinAddress";
import { useOptimisticSave } from "@/hooks/useOptimisticSave";
import { usePinView } from "@/hooks/usePinView";
import { usePinShare } from "@/hooks/usePinShare";
import { formatTime, parsePoint } from "@/lib/utils";
import { categorySlug } from "@/lib/utils/category";
import { useAuth } from "@/store/auth";
import type { PinDetail } from "@/lib/types";

interface PinPageClientProps {
  initialPin: PinDetail;
}

export default function PinPageClient({ initialPin }: PinPageClientProps) {
  const router = useRouter();
  const { user } = useAuth();
  const { categories } = useCategories();
  const [pin, setPin] = useState<PinDetail>(initialPin);
  const [index, setIndex] = useState(0);
  const [lightbox, setLightbox] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [collectionOpen, setCollectionOpen] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  const { saved, saving, handleSave } = useOptimisticSave(pin.id);
  const view = usePinView(pin.id, pin.user_id);

  // Applies the counted total to local pin state once per fetch. The parent
  // remounts via route change on the permalink page, so no pinId guard is
  // needed here.
  useEffect(() => {
    if (!view) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- merge the freshly counted total into local pin state once per fetch
    setPin((prev) => ({ ...prev, views: view.views }));
  }, [view]);

  const photo = pin.photos[index];
  const count = pin.photos.length;
  const point = useMemo(() => parsePoint(pin.location), [pin.location]);

  const isOwner =
    user !== null && (user.id === pin.user_id || user.role === "admin" || user.role === "owner");

  const address = usePinAddress(point);

  const { copied, handleShare, goDirections } = usePinShare(pin.caption, pin.id, point);

  const handleUpdated = async () => {
    try {
      const refreshed = await fetchPin(pin.id);
      setPin(refreshed);
    } catch {
      // the edit sheet already confirmed the update server-side
    }
  };

  const name = pin.caption?.trim();

  return (
    <>
      <Navbar backHref="/" backLabel="Back to map" />

      <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
        {/* Hero */}
        <div className="overflow-hidden rounded-2xl bg-zinc-100 shadow-sm ring-1 ring-zinc-200/60 dark:bg-zinc-900 dark:ring-zinc-800">
          {photo ? (
            <button
              type="button"
              onClick={() => setLightbox(true)}
              className="block w-full cursor-zoom-in"
              aria-label="View photo full screen"
            >
              <img
                src={photo.photo_url}
                alt={name ?? "Pin photo"}
                className="max-h-[60dvh] w-full object-cover"
              />
            </button>
          ) : (
            <div className="flex aspect-[16/10] items-center justify-center text-zinc-300 dark:text-zinc-600">
              <PinIcon className="h-10 w-10" />
            </div>
          )}

          {count > 1 && (
            <div className="flex gap-2 overflow-x-auto p-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
              {pin.photos.map((p, i) => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => setIndex(i)}
                  aria-label={`Photo ${i + 1} of ${count}`}
                  aria-pressed={i === index}
                  className={`h-14 w-14 shrink-0 overflow-hidden rounded-lg border-2 transition-colors ${
                    i === index
                      ? "border-sky-500"
                      : "border-transparent hover:border-zinc-300 dark:hover:border-zinc-600"
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
          )}
        </div>

        {/* Title + actions */}
        <div className="mt-5 flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="text-2xl font-bold leading-tight text-zinc-900 dark:text-zinc-100">
              {name ?? "Untitled"}
            </h1>
            <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-zinc-500 dark:text-zinc-400">
              {pin.category && (
                <span className="font-medium text-zinc-700 dark:text-zinc-300">
                  {pin.category}
                </span>
              )}
              {pin.username && (
                <Link
                  href={pin.user_id ? `/users/${pin.user_id}` : "#"}
                  className="flex items-center gap-1 hover:underline"
                >
                  <PersonIcon />
                  {pin.username}
                </Link>
              )}
              <span className="text-zinc-300 dark:text-zinc-600">·</span>
              <time dateTime={pin.created_at}>{formatTime(pin.created_at)}</time>
              <span className="text-zinc-300 dark:text-zinc-600">·</span>
              <PinViews views={pin.views} />
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              onClick={handleSave}
              disabled={saving}
              aria-pressed={saved}
              className="flex items-center gap-1.5 rounded-full bg-rose-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-rose-700 disabled:opacity-60"
            >
              <BookmarkIcon filled={saved} className="h-4 w-4" />
              {saved ? "Saved" : "Save"}
            </button>
            <button
              type="button"
              onClick={handleShare}
              className="rounded-full border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              {copied ? "Copied!" : "Share"}
            </button>
            <button
              type="button"
              onClick={goDirections}
              disabled={!point}
              className="rounded-full border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:opacity-40 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Directions
            </button>
          </div>
        </div>

        {/* Description / address */}
        <div className="mt-6 space-y-2 text-sm leading-relaxed text-zinc-700 dark:text-zinc-300">
          {name && <p className="whitespace-pre-wrap">{pin.caption}</p>}
          <p className="text-zinc-500 dark:text-zinc-400">
            {address ?? (point ? "Finding address…" : "Location pinned on the map")}
          </p>
        </div>

        {/* Owner + collection actions */}
        <div className="mt-6 flex flex-wrap items-center gap-2">
          {isOwner && (
            <button
              type="button"
              onClick={() => setEditOpen(true)}
              className="rounded-full border border-zinc-300 px-4 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Edit pin
            </button>
          )}
          <button
            type="button"
            onClick={() => setCollectionOpen(true)}
            className="rounded-full border border-zinc-300 px-4 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            Add to collection
          </button>
          {user && user?.id !== pin.user_id && (
            <button
              type="button"
              onClick={() => setReportOpen(true)}
              className="rounded-full border border-zinc-300 px-4 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Report
            </button>
          )}
          {isOwner && user?.id === pin.user_id && (
            <Link
              href="/"
              className="rounded-full border border-zinc-300 px-4 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              View on map
            </Link>
          )}
        </div>

        {/* Comments */}
        <div className="mt-8 border-t border-zinc-100 pt-6 dark:border-zinc-800">
          <CommentsSection pinId={pin.id} />
        </div>
      </div>

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
          categories={
            categories.length > 0
              ? categories
              : [
                  {
                    id: pin.category_id,
                    name: pin.category ?? "Other",
                    slug: categorySlug(pin.category ?? "Other"),
                  },
                ]
          }
          onClose={() => setEditOpen(false)}
          onUpdated={handleUpdated}
          onDeleted={() => router.push("/")}
        />
      )}

      {collectionOpen && (
        <AddToCollectionSheet pinId={pin.id} onClose={() => setCollectionOpen(false)} />
      )}

      {reportOpen && (
        <ReportSheet pinId={pin.id} onClose={() => setReportOpen(false)} />
      )}
    </>
  );
}