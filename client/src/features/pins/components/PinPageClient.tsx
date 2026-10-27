"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Navbar from "@/shared/layout/Navbar";
import { PhotoThumbStrip } from "@/features/photos";
import { PinActionSheets } from "./PinActionSheets";
import { CommentsSection } from "@/features/comments";
import PinViews from "./PinViews";
import { BookmarkIcon, PersonIcon, PinIcon } from "@/shared/ui/icons";
import { useCategories } from "@/features/map";
import { useEditablePin } from "../hooks/useEditablePin";
import { usePinAddress } from "../hooks/usePinAddress";
import { useOptimisticSave } from "@/features/collections";
import { usePinShare } from "../hooks/usePinShare";
import { isPinOwner } from "../lib/pinPermission";
import { formatTime } from "@/shared/utils";
import { useAuth } from "@/features/auth";
import type { PinDetail } from "@/shared/types/pin";

interface PinPageClientProps {
  initialPin: PinDetail;
}

export default function PinPageClient({ initialPin }: PinPageClientProps) {
  const router = useRouter();
  const { user } = useAuth();
  const { categories } = useCategories();
  const {
    pin,
    index,
    setIndex,
    count,
    heroSrc,
    lightboxSrc,
    point,
    name,
    handleUpdated,
  } = useEditablePin(initialPin);
  const [lightbox, setLightbox] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [collectionOpen, setCollectionOpen] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  const { saved, saving, error: saveError, handleSave } = useOptimisticSave(pin.id);

  const isOwner = isPinOwner(user, pin.user_id);

  const address = usePinAddress(point);

  const { copied, handleShare, goDirections } = usePinShare(pin.caption, pin.id, point);

  return (
    <>
      <Navbar backHref="/" backLabel="Back to map" />

      <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
        {/* Hero */}
        <div className="overflow-hidden rounded-2xl bg-zinc-100 shadow-sm ring-1 ring-zinc-200/60 dark:bg-zinc-900 dark:ring-zinc-800">
          {heroSrc ? (
            <button
              type="button"
              onClick={() => setLightbox(true)}
              className="block w-full cursor-zoom-in"
              aria-label="View photo full screen"
            >
              <img
                src={heroSrc}
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
              <PhotoThumbStrip
                photos={pin.photos}
                index={index}
                onSelect={setIndex}
                thumbBoxClass="h-14 w-14 rounded-lg"
              />
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
              {pin.username ? (
                <Link
                  href={pin.user_id ? `/users/${pin.user_id}` : "#"}
                  className="flex items-center gap-1 hover:underline"
                >
                  <PersonIcon />
                  {pin.username}
                </Link>
              ) : (
                <span className="flex items-center gap-1 text-zinc-400 dark:text-zinc-500">
                  <PersonIcon />
                  deleted user
                </span>
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
            {saveError && (
              <p role="alert" className="text-xs text-rose-600 dark:text-rose-400">
                {saveError}
              </p>
            )}
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

      <PinActionSheets
        pin={pin}
        categories={categories}
        lightbox={lightbox}
        lightboxSrc={lightboxSrc}
        name={name}
        count={count}
        index={index}
        editOpen={editOpen}
        collectionOpen={collectionOpen}
        reportOpen={reportOpen}
        onNavigate={setIndex}
        onCloseLightbox={() => setLightbox(false)}
        onCloseEdit={() => setEditOpen(false)}
        onUpdated={handleUpdated}
        onDeleted={() => router.push("/")}
        onCloseCollection={() => setCollectionOpen(false)}
        onCloseReport={() => setReportOpen(false)}
      />
    </>
  );
}