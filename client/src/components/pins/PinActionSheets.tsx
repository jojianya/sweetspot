"use client";

import PhotoLightbox from "./PhotoLightbox";
import PinEditSheet from "./PinEditSheet";
import AddToCollectionSheet from "./AddToCollectionSheet";
import ReportSheet from "./ReportSheet";
import { categorySlug } from "@/lib/utils/category";
import type { Category } from "@/shared/types/category"
import type { PinDetail } from "@/shared/types/pin";

/**
 * Lightbox plus edit/collection/report sheets shared by the panel and the
 * permalink page. The edit sheet synthesizes the pin's own category when the
 * category list is empty. Delete destination stays with each shell: the
 * panel closes into onDeleted(id), the page navigates home.
 */
export function PinActionSheets({
  pin,
  categories,
  lightbox,
  lightboxSrc,
  name,
  count,
  index,
  editOpen,
  collectionOpen,
  reportOpen,
  onNavigate,
  onCloseLightbox,
  onCloseEdit,
  onUpdated,
  onDeleted,
  onCloseCollection,
  onCloseReport,
}: {
  pin: PinDetail;
  categories: Category[];
  lightbox: boolean;
  lightboxSrc: string;
  name: string | undefined;
  count: number;
  index: number;
  editOpen: boolean;
  collectionOpen: boolean;
  reportOpen: boolean;
  onNavigate: (index: number) => void;
  onCloseLightbox: () => void;
  onCloseEdit: () => void;
  onUpdated: (pin: PinDetail) => void;
  onDeleted: (id: string) => void;
  onCloseCollection: () => void;
  onCloseReport: () => void;
}) {
  return (
    <>
      {lightbox && lightboxSrc ? (
        <PhotoLightbox
          src={lightboxSrc}
          alt={name ?? "Pin photo"}
          count={count}
          index={index}
          onNavigate={onNavigate}
          onClose={onCloseLightbox}
        />
      ) : null}

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
          onClose={onCloseEdit}
          onUpdated={onUpdated}
          onDeleted={onDeleted}
        />
      )}

      {collectionOpen && (
        <AddToCollectionSheet pinId={pin.id} onClose={onCloseCollection} />
      )}

      {reportOpen && (
        <ReportSheet pinId={pin.id} onClose={onCloseReport} />
      )}
    </>
  );
}
